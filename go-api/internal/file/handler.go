package file

import (
	"net/http"
	"path/filepath"
	"strings"

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

// UploadFilesHandler stores one or more original files and registers their metadata.
// Multipart fields: files (repeatable), fileType (optional).
func UploadFilesHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 500<<20)
		if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multipart upload: " + err.Error()})
			return
		}
		headers := c.Request.MultipartForm.File["files"]
		if len(headers) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "At least one 'files' upload is required"})
			return
		}
		if len(headers) > 20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "A maximum of 20 files can be uploaded at once"})
			return
		}

		fileType := strings.TrimSpace(c.PostForm("fileType"))
		if fileType == "" {
			fileType = "evidence"
		}
		created := make([]models.ProjectFile, 0, len(headers))
		for _, header := range headers {
			fileID := uuid.NewString()
			storagePath, size, checksum, mimeType, err := saveOriginal(projectID, fileID, header)
			if err != nil {
				for _, item := range created {
					removeOriginal(item.StoragePath)
				}
				c.JSON(http.StatusBadRequest, gin.H{"error": header.Filename + ": " + err.Error()})
				return
			}
			item := models.ProjectFile{
				ID: fileID, ProjectID: projectID,
				FileName: filepath.Base(header.Filename), FileType: fileType,
				FileURL:  "/api/projects/" + projectID + "/files/" + fileID + "/content",
				MimeType: mimeType, Size: size, SHA256: checksum, StoragePath: storagePath,
			}
			if err := fbClient.CreateFile(c.Request.Context(), &item); err != nil {
				removeOriginal(storagePath)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
				return
			}
			created = append(created, item)
		}
		c.JSON(http.StatusCreated, gin.H{"files": created})
	}
}

func DownloadFileHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		files, err := fbClient.GetFilesByProject(c.Request.Context(), c.Param("projectId"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files"})
			return
		}
		for _, item := range files {
			if item.ID == c.Param("fileId") {
				if item.StoragePath == "" {
					c.JSON(http.StatusNotFound, gin.H{"error": "Original file is not stored by this server"})
					return
				}
				c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(filepath.Base(item.FileName), `"`, "")+`"`)
				c.Header("Content-Type", item.MimeType)
				c.Header("X-Content-Type-Options", "nosniff")
				c.File(item.StoragePath)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
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
