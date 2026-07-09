package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"recoverpack-server/go-api/internal/ai"
	"recoverpack-server/go-api/internal/evidence"
	"recoverpack-server/go-api/internal/file"
	"recoverpack-server/go-api/internal/firebase"
	pckg "recoverpack-server/go-api/internal/package"
	"recoverpack-server/go-api/internal/project"
	"recoverpack-server/go-api/internal/timeline"
)

func main() {
	// 1. Load env variables from .env if present
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found, using system environment variables.")
	}

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 2. Initialize Firebase Client (with in-memory mock fallback)
	fbClient, err := firebase.NewClient()
	if err != nil {
		log.Fatalf("[CRITICAL] Failed to initialize database: %v", err)
	}
	defer fbClient.Close()

	// 3. Initialize AI Service Client
	aiClient := ai.NewClient()

	// 4. Initialize Gin Router
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 5. Setup CORS Middleware
	r.Use(corsMiddleware())

	// 6. Public/Internal Health Endpoints
	r.GET("/health", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dbStatus := "healthy"
		if fbClient.IsMock() {
			dbStatus = "mock-active"
		} else {
			// Double check firestore availability
			_, err := fbClient.GetProject(ctx, "ping-test")
			if err != nil && err.Error() != "project not found" {
				dbStatus = "degraded"
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status":      "healthy",
			"service":     "go-api",
			"database":    dbStatus,
			"ai_endpoint": os.Getenv("AI_SERVICE_URL"),
		})
	})

	// 7. Core API Endpoints
	api := r.Group("/api")
	{
		// Project Endpoints
		api.POST("/projects", project.CreateProjectHandler(fbClient))
		api.GET("/projects/:projectId", project.GetProjectHandler(fbClient))

		// File Upload Metadata Endpoints
		api.POST("/projects/:projectId/files", file.AddFileHandler(fbClient))
		api.GET("/projects/:projectId/files", file.GetFilesHandler(fbClient))

		// Evidence Endpoints (AI Analysis)
		api.POST("/projects/:projectId/analyze", evidence.AnalyzeProjectHandler(fbClient, aiClient))
		api.GET("/projects/:projectId/evidence", evidence.GetEvidenceHandler(fbClient))
		api.PATCH("/projects/:projectId/evidence/:evidenceId", evidence.UpdateEvidenceHandler(fbClient))

		// Timeline Endpoints
		api.POST("/projects/:projectId/timeline", timeline.SaveTimelineHandler(fbClient, aiClient))
		api.GET("/projects/:projectId/timeline", timeline.GetTimelineHandler(fbClient))

		// Package / Report Endpoints
		api.POST("/projects/:projectId/package", pckg.GeneratePackageHandler(fbClient))
		api.GET("/projects/:projectId/download", pckg.DownloadPackageHandler(fbClient))
	}

	// 8. Start HTTP Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("[SERVER] Starting Go API Server on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[CRITICAL] Server failed to start: %v", err)
	}
}

// corsMiddleware provides standard CORS permissions for development
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(24)
			return
		}

		c.Next()
	}
}
