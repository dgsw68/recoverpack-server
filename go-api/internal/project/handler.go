package project

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"recoverpack-server/go-api/internal/auth"
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
		user, ok := auth.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		projectID := uuid.New().String()
		p := &models.Project{
			ID:         projectID,
			UserID:     user.ID,
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

// ListProjectsHandler returns all projects owned by the authenticated user
func ListProjectsHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := auth.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		projects, err := fbClient.ListProjectsByUser(c.Request.Context(), user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"projects": projects})
	}
}

// UpdateReporterInfoRequest carries the 피해자 정보 and 간접지원 체크리스트 fields
// used to prefill the official 자연재난 피해신고서. All fields are optional so the
// UI can save this step separately from project creation.
type UpdateReporterInfoRequest struct {
	ReporterName    *string                 `json:"reporterName"`
	ReporterPhone   *string                 `json:"reporterPhone"`
	ReporterAddress *string                 `json:"reporterAddress"`
	ResidenceType   *string                 `json:"residenceType"`
	IndirectSupport *models.IndirectSupport `json:"indirectSupport"`
}

// UpdateReporterInfoHandler updates the reporter/피해자 정보 and 간접지원 체크리스트
// for a project so the official-form PDF can be prefilled.
func UpdateReporterInfoHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		var req UpdateReporterInfoRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
			return
		}

		if err := fbClient.UpdateReporterInfo(c.Request.Context(), projectID, req.ReporterName, req.ReporterPhone, req.ReporterAddress, req.ResidenceType, req.IndirectSupport); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update reporter info: " + err.Error()})
			return
		}

		p, err := fbClient.GetProject(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch updated project: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, p)
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
