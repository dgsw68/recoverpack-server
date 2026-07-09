package timeline

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/ai"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

type TimelineEventInput struct {
	Title       string `json:"title" binding:"required"`
	Description string `json:"description" binding:"required"`
	EventDate   string `json:"eventDate" binding:"required"`
}

type SaveTimelineRequest struct {
	AutoGenerate bool                 `json:"autoGenerate"`
	Events       []TimelineEventInput `json:"events"`
}

// SaveTimelineHandler handles creating, updating or auto-generating chronological timeline events
func SaveTimelineHandler(fbClient *firebase.Client, aiClient *ai.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		// 1. Verify project exists
		if _, err := fbClient.GetProject(ctx, projectID); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		var req SaveTimelineRequest
		// We ignore binding errors on JSON in case they pass empty body for pure auto-generation
		_ = c.ShouldBindJSON(&req)

		var finalEvents []models.TimelineEvent

		if req.AutoGenerate || len(req.Events) == 0 {
			// Auto-generation path via AI
			evidence, err := fbClient.GetEvidenceByProject(ctx, projectID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch evidence for timeline generation: " + err.Error()})
				return
			}

			if len(evidence) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "No evidence found to generate a timeline. Run analysis first."})
				return
			}

			aiEvents, err := aiClient.GenerateTimeline(ctx, projectID, evidence)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate timeline via AI: " + err.Error()})
				return
			}
			finalEvents = aiEvents
		} else {
			// Custom manual save path (User edit overrides)
			now := time.Now()
			for i, e := range req.Events {
				finalEvents = append(finalEvents, models.TimelineEvent{
					ID:          fmt.Sprintf("time_%d_%d", now.UnixNano(), i),
					ProjectID:   projectID,
					Title:       e.Title,
					Description: e.Description,
					EventDate:   e.EventDate,
					CreatedAt:   now,
				})
			}
		}

		// Save final timeline to database
		if err := fbClient.SaveTimelineEvents(ctx, projectID, finalEvents); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save timeline: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, finalEvents)
	}
}

// GetTimelineHandler retrieves chronological timeline events for a project
func GetTimelineHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		events, err := fbClient.GetTimelineByProject(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch timeline: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, events)
	}
}
