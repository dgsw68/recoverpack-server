package pckg

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

// GeneratePackageHandler generates mock submission-ready evidence package contents and details.
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

		// 2. Formulate mock download URL and contents
		// In a later iteration, this will trigger a worker to bundle the Firestore records and Storage images into a PDF and ZIP
		downloadURL := fmt.Sprintf("http://localhost:8080/api/projects/%s/download", projectID)
		contents := []string{
			"RecoverPack_종합피해보고서.pdf",
			"RecoverPack_증빙사진대장.zip",
			"RecoverPack_견적서_및_영수증_모음.pdf",
		}

		pkgInfo := &models.PackageInfo{
			ProjectID:   projectID,
			PackageURL:  downloadURL,
			Contents:    contents,
			GeneratedAt: time.Now(),
		}

		// 3. Save package info
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
			"message":     "Evidence package generated successfully! ready for download.",
			"note":        "This is a submission support package and is not an official government or legal document.",
		})
	}
}

// DownloadPackageHandler returns download URL and status
func DownloadPackageHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		pkgInfo, err := fbClient.GetPackage(ctx, projectID)
		if err != nil {
			// On-the-fly generation for robust UX if not pre-generated
			project, pErr := fbClient.GetProject(ctx, projectID)
			if pErr != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
				return
			}

			downloadURL := fmt.Sprintf("http://localhost:8080/api/projects/%s/download", projectID)
			contents := []string{
				"RecoverPack_종합피해보고서.pdf",
				"RecoverPack_증빙사진대장.zip",
				"RecoverPack_견적서_및_영수증_모음.pdf",
			}

			pkgInfo = &models.PackageInfo{
				ProjectID:   projectID,
				PackageURL:  downloadURL,
				Contents:    contents,
				GeneratedAt: time.Now(),
			}

			_ = fbClient.SavePackage(ctx, pkgInfo)
			_ = project // Avoid unused variable check if needed
		}

		c.JSON(http.StatusOK, gin.H{
			"projectId":   pkgInfo.ProjectID,
			"status":      "ready",
			"packageUrl":  pkgInfo.PackageURL,
			"contents":    pkgInfo.Contents,
			"generatedAt": pkgInfo.GeneratedAt,
			"note":        "제출용 보조 증빙 자료이며 법적 효력이 있는 공식 문서가 아닙니다.",
		})
	}
}
