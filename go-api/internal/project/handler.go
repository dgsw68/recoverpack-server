package project

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

type CreateProjectRequest struct {
	DamageType string `json:"damageType" binding:"required"`
	Title      string `json:"title" binding:"required"`
	Location   string `json:"location" binding:"required"`
	OccurredAt string `json:"occurredAt" binding:"required"`
}

// CreateProjectHandler creates a new damage project in the store
func CreateProjectHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
			return
		}

		projectID := uuid.New().String()
		p := &models.Project{
			ID:         projectID,
			DamageType: req.DamageType,
			Title:      req.Title,
			Location:   req.Location,
			OccurredAt: req.OccurredAt,
		}

		if err := fbClient.CreateProject(c.Request.Context(), p); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create project: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"projectId": projectID,
			"message":   "Project created successfully",
		})
	}
}

// GetProjectHandler retrieves project metadata
func GetProjectHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		p, err := fbClient.GetProject(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		c.JSON(http.StatusOK, p)
	}
}
