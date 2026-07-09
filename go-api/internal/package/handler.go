package pckg

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/firebase"
)

// GeneratePackageHandler creates a real ZIP file from the data currently stored for a project.
func GeneratePackageHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		// 1. Verify project exists
		project, err := fbClient.GetProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		files, err := fbClient.GetFilesByProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch project files: " + err.Error()})
			return
		}
		evidence, err := fbClient.GetEvidenceByProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch evidence: " + err.Error()})
			return
		}
		timeline, err := fbClient.GetTimelineByProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch timeline: " + err.Error()})
			return
		}

		pkgInfo, err := Generate(project, files, evidence, timeline)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate package: " + err.Error()})
			return
		}
		if err := fbClient.SavePackage(ctx, pkgInfo); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save package info: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"projectId":   projectID,
			"title":       project.Title,
			"packageUrl":  pkgInfo.PackageURL,
			"contents":    pkgInfo.Contents,
			"generatedAt": pkgInfo.GeneratedAt,
			"message":     "Evidence package generated successfully and is ready for download.",
			"note":        "This is a submission support package and is not an official government or legal document.",
		})
	}
}

// DownloadPackageHandler streams the generated ZIP file.
func DownloadPackageHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		_, err := fbClient.GetPackage(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Package not generated. Call the package generation endpoint first."})
			return
		}

		zipPath, err := PackagePath(projectID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid project ID"})
			return
		}
		if _, err := os.Stat(zipPath); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Generated package file is unavailable. Regenerate the package."})
			return
		}

		c.Header("Content-Disposition", `attachment; filename="recoverpack_evidence_package.zip"`)
		c.Header("X-Content-Type-Options", "nosniff")
		c.File(zipPath)
	}
}
