package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"recoverpack-server/go-api/internal/ai"
	"recoverpack-server/go-api/internal/auth"
	"recoverpack-server/go-api/internal/disaster"
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
		if len(os.Getenv("AUTH_SECRET")) < 32 {
			log.Fatal("[CRITICAL] AUTH_SECRET must be at least 32 characters in release mode.")
		}
	}

	// 2. Initialize Firebase Client (with in-memory mock fallback)
	fbClient, err := firebase.NewClient()
	if err != nil {
		log.Fatalf("[CRITICAL] Failed to initialize database: %v", err)
	}
	defer fbClient.Close()

	// 3. Initialize AI Service Client
	aiClient := ai.NewClient()

	// 3b. Load 긴급재난문자 disaster alert data from local CSV export
	// (stand-in until data.go.kr API approval comes through).
	disasterCSVPath := os.Getenv("DISASTER_ALERT_CSV")
	if disasterCSVPath == "" {
		disasterCSVPath = "data/disaster_alerts.csv"
	}
	disasterStore, err := disaster.LoadStore(disasterCSVPath)
	if err != nil {
		log.Printf("[DISASTER] Failed to load disaster alert CSV from %s: %v", disasterCSVPath, err)
		disasterStore = &disaster.Store{}
	} else {
		log.Printf("[DISASTER] Loaded %d disaster alerts from %s", disasterStore.Len(), disasterCSVPath)
	}

	// 3c. 기상청_기상특보 조회서비스 client ("공식 재난상황 근거 자동 연결").
	// Works with an empty key too: FetchAlerts then just returns no alerts
	// instead of erroring, so the rest of the service is unaffected while
	// the key is pending.
	weatherClient := disaster.NewWeatherClient(
		os.Getenv("KMA_SPECIAL_WEATHER_API_KEY"),
		os.Getenv("KMA_API_BASE_URL"),
	)
	if !weatherClient.Enabled() {
		log.Println("[WEATHER] KMA_SPECIAL_WEATHER_API_KEY not set; /api/disaster/weather-alerts will return empty results.")
	}

	// 3d. 기상청_지상(종관, ASOS) 시간자료 조회서비스 client ("실측 기상 근거
	// 자동 연결"). Reuses the same general KMA auth key as weatherClient.
	// Unlike forecast APIs this has no recency limit, so it can support
	// evidence for damage reported long after the fact.
	asosClient := disaster.NewAsosClient(
		os.Getenv("KMA_SPECIAL_WEATHER_API_KEY"),
		os.Getenv("KMA_ASOS_API_BASE_URL"),
	)
	if !asosClient.Enabled() {
		log.Println("[ASOS] KMA_SPECIAL_WEATHER_API_KEY not set; /api/disaster/asos-observations will return empty results.")
	}

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
			if err != nil && status.Code(err) != codes.NotFound {
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
		api.POST("/auth/register", auth.RegisterHandler(fbClient))
		api.POST("/auth/login", auth.LoginHandler(fbClient))
		api.GET("/disaster-alerts", disaster.ListAlertsHandler(disasterStore))
		api.GET("/disaster/weather-alerts", disaster.WeatherAlertsHandler(weatherClient))
		api.GET("/disaster/asos-observations", disaster.AsosObservationsHandler(asosClient))

		protected := api.Group("")
		protected.Use(auth.RequireAuth(fbClient))
		protected.GET("/auth/me", auth.MeHandler())
		protected.POST("/projects", project.CreateProjectHandler(fbClient))
		protected.GET("/projects", project.ListProjectsHandler(fbClient))
		protected.GET("/me/packages", pckg.ListMyPackagesHandler(fbClient))

		ownedProject := protected.Group("/projects/:projectId")
		ownedProject.Use(auth.RequireProjectOwner(fbClient))
		{
			ownedProject.GET("", project.GetProjectHandler(fbClient))
			ownedProject.PATCH("/reporter", project.UpdateReporterInfoHandler(fbClient))
			ownedProject.POST("/files", file.AddFileHandler(fbClient))
			ownedProject.POST("/uploads", file.UploadFilesHandler(fbClient))
			ownedProject.GET("/files", file.GetFilesHandler(fbClient))
			ownedProject.GET("/files/:fileId/content", file.DownloadFileHandler(fbClient))
			ownedProject.POST("/analyze", evidence.AnalyzeProjectHandler(fbClient, aiClient))
			ownedProject.GET("/evidence", evidence.GetEvidenceHandler(fbClient))
			ownedProject.PATCH("/evidence/:evidenceId", evidence.UpdateEvidenceHandler(fbClient))
			ownedProject.POST("/timeline", timeline.SaveTimelineHandler(fbClient, aiClient, disasterStore, weatherClient, asosClient))
			ownedProject.GET("/timeline", timeline.GetTimelineHandler(fbClient))
			ownedProject.POST("/package", pckg.GeneratePackageHandler(fbClient))
			ownedProject.GET("/download", pckg.DownloadPackageHandler(fbClient))
		}
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
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
