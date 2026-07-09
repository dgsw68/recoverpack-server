package pckg

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/auth"
	"recoverpack-server/go-api/internal/firebase"
)

// MyPackageItem summarizes one project's evidence-package status for the
// 마이페이지 record list ("내 ZIP 생성 기록").
type MyPackageItem struct {
	ProjectID         string `json:"projectId"`
	PackageID         string `json:"packageId"`
	Title             string `json:"title"`
	DamageType        string `json:"damageType"`
	Location          string `json:"location"`
	OccurredAt        string `json:"occurredAt"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
	Status            string `json:"status"`
	FileCount         int    `json:"fileCount"`
	EvidenceCount     int    `json:"evidenceCount"`
	TimelineCount     int    `json:"timelineCount"`
	DownloadAvailable bool   `json:"downloadAvailable"`
}

// ListMyPackagesHandler returns every project owned by the current user,
// enriched with file/evidence/timeline counts and package generation status.
func ListMyPackagesHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		user, ok := auth.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			return
		}

		projects, err := fbClient.ListProjectsByUser(ctx, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list projects: " + err.Error()})
			return
		}

		items := make([]MyPackageItem, 0, len(projects))
		for _, p := range projects {
			files, err := fbClient.GetFilesByProject(ctx, p.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files: " + err.Error()})
				return
			}
			evidence, err := fbClient.GetEvidenceByProject(ctx, p.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch evidence: " + err.Error()})
				return
			}
			timeline, err := fbClient.GetTimelineByProject(ctx, p.ID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch timeline: " + err.Error()})
				return
			}

			item := MyPackageItem{
				ProjectID:     p.ID,
				PackageID:     p.ID,
				Title:         p.Title,
				DamageType:    p.DamageType,
				Location:      p.Location,
				OccurredAt:    p.OccurredAt,
				CreatedAt:     p.CreatedAt.Format(time.RFC3339),
				UpdatedAt:     p.CreatedAt.Format(time.RFC3339),
				Status:        "draft",
				FileCount:     len(files),
				EvidenceCount: len(evidence),
				TimelineCount: len(timeline),
			}

			if pkg, err := fbClient.GetPackage(ctx, p.ID); err == nil {
				item.Status = "completed"
				item.UpdatedAt = pkg.GeneratedAt.Format(time.RFC3339)
				if zipPath, pathErr := PackagePath(p.ID); pathErr == nil {
					if _, statErr := os.Stat(zipPath); statErr == nil {
						item.DownloadAvailable = true
					}
				}
			}

			items = append(items, item)
		}

		c.JSON(http.StatusOK, gin.H{"items": items})
	}
}
