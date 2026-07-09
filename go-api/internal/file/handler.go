package file

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

type AddFileRequest struct {
	FileName string `json:"fileName" binding:"required"`
	FileType string `json:"fileType" binding:"required"`
	FileURL  string `json:"fileUrl" binding:"required"`
	MimeType string `json:"mimeType" binding:"required"`
}

// AddFileHandler registers file metadata for a project
func AddFileHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		// Check if project exists
		if _, err := fbClient.GetProject(c.Request.Context(), projectID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		var req AddFileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
			return
		}

		fileID := uuid.New().String()
		f := &models.ProjectFile{
			ID:        fileID,
			ProjectID: projectID,
			FileName:  req.FileName,
			FileType:  req.FileType,
			FileURL:   req.FileURL,
			MimeType:  req.MimeType,
		}

		if err := fbClient.CreateFile(c.Request.Context(), f); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register file: " + err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"fileId":    fileID,
			"projectId": projectID,
			"message":   "File metadata registered successfully",
		})
	}
}

// GetFilesHandler lists registered file metadata for a project
func GetFilesHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		files, err := fbClient.GetFilesByProject(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, files)
	}
}
