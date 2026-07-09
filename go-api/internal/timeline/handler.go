package timeline

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/ai"
	"recoverpack-server/go-api/internal/disaster"
	"recoverpack-server/go-api/internal/firebase"
	"recoverpack-server/go-api/internal/models"
)

// disasterAlertMatchWindow bounds how far a 긴급재난문자 alert's sent time may
// drift from the project's reported occurredAt and still be considered
// related. Disaster events (e.g. prolonged heavy rain) can span a few days.
const disasterAlertMatchWindow = 72 * time.Hour

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
func SaveTimelineHandler(fbClient *firebase.Client, aiClient *ai.Client, disasterStore *disaster.Store, weatherClient *disaster.WeatherClient, asosClient *disaster.AsosClient) gin.HandlerFunc {
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

			// 2. Cross-reference 행정안전부 긴급재난문자 alerts for this project's
			// location/date so the timeline shows the official disaster warning
			// that was in effect, not just the user's own evidence.
			if disasterStore != nil {
				alerts := disasterStore.MatchByLocationAndDate(project.Location, project.OccurredAt, disasterAlertMatchWindow)
				now := time.Now()
				for i, alert := range alerts {
					eventDate := alert.CreatedAt
					if sentAt, ok := disaster.ParseAlertTime(alert.CreatedAt); ok {
						eventDate = sentAt.Format("2006-01-02 15:04")
					}
					finalEvents = append(finalEvents, models.TimelineEvent{
						ID:          fmt.Sprintf("alert_%d_%d", now.UnixNano(), i),
						ProjectID:   projectID,
						Title:       "긴급재난문자 발송 (" + alert.DisasterType + ")",
						Description: alert.Message,
						EventDate:   eventDate,
						CreatedAt:   now,
					})
				}
				// 3. Cross-reference the 기상청_기상특보 조회서비스 ("공식
				// 재난상황 근거 자동 연결") for the same location/date. Never
				// blocks the response: FetchAlerts returns an empty slice on
				// any lookup failure.
				if weatherClient != nil {
					day := strings.SplitN(strings.TrimSpace(project.OccurredAt), " ", 2)[0]
					weatherAlerts := weatherClient.FetchAlerts(ctx, project.Location, day)
					now := time.Now()
					for i, alert := range weatherAlerts {
						eventDate := alert.AnnouncedAt
						if t, err := time.Parse("2006-01-02T15:04:05", alert.AnnouncedAt); err == nil {
							eventDate = t.Format("2006-01-02 15:04")
						}
						finalEvents = append(finalEvents, models.TimelineEvent{
							ID:          fmt.Sprintf("weather_%d_%d", now.UnixNano(), i),
							ProjectID:   projectID,
							Title:       "기상특보 발표 (" + alert.Title + ")",
							Description: alert.Content + " [출처: " + alert.Source + "]",
							EventDate:   eventDate,
							CreatedAt:   now,
						})
					}
				}

				// 4. Cross-reference the 기상청_지상(종관,ASOS) 시간자료
				// 조회서비스 ("실측 기상 근거 자동 연결") for the same
				// location/date. Unlike the forecast/특보 APIs this is actual
				// recorded weather with no recency limit. Summarized into a
				// single event (not one per hour) to keep the timeline
				// readable. Never blocks the response: FetchObservations
				// returns an empty slice on any lookup failure.
				if asosClient != nil {
					day := strings.SplitN(strings.TrimSpace(project.OccurredAt), " ", 2)[0]
					observations := asosClient.FetchObservations(ctx, project.Location, day)
					if summary, ok := summarizeAsosObservations(observations); ok {
						now := time.Now()
						finalEvents = append(finalEvents, models.TimelineEvent{
							ID:          fmt.Sprintf("asos_%d", now.UnixNano()),
							ProjectID:   projectID,
							Title:       "기상 실측 자료 (ASOS)",
							Description: summary,
							EventDate:   day + " 00:00",
							CreatedAt:   now,
						})
					}
				}

				sortEventsByDate(finalEvents)
			}
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

// sortEventsByDate orders events chronologically by EventDate. Events whose
// date can't be parsed are pushed to the end, in their original order.
func sortEventsByDate(events []models.TimelineEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		ti, iok := disaster.ParseAlertTime(events[i].EventDate)
		tj, jok := disaster.ParseAlertTime(events[j].EventDate)
		if !iok {
			return false
		}
		if !jok {
			return true
		}
		return ti.Before(tj)
	})
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

// summarizeAsosObservations condenses a day of hourly ASOS observations into
// a single human-readable line (total rainfall, peak hourly rainfall, peak
// wind), so the timeline gets one evidence entry instead of up to 24. Returns
// ok=false if there's nothing worth reporting.
func summarizeAsosObservations(observations []disaster.AsosObservation) (string, bool) {
	if len(observations) == 0 {
		return "", false
	}

	var totalRain, peakRain, peakWind float64
	var peakRainHour, peakWindHour, station string
	for _, o := range observations {
		totalRain += o.PrecipitationMM
		if o.PrecipitationMM > peakRain {
			peakRain = o.PrecipitationMM
			peakRainHour = o.Time
		}
		if o.WindSpeedMs > peakWind {
			peakWind = o.WindSpeedMs
			peakWindHour = o.Time
		}
		if station == "" {
			station = o.StationName
		}
	}

	parts := []string{fmt.Sprintf("관측소: %s", station), fmt.Sprintf("일 강수량 합계 %.1fmm", totalRain)}
	if peakRain > 0 {
		parts = append(parts, fmt.Sprintf("시간당 최대 강수량 %.1fmm(%s)", peakRain, peakRainHour))
	}
	if peakWind > 0 {
		parts = append(parts, fmt.Sprintf("최대 풍속 %.1fm/s(%s)", peakWind, peakWindHour))
	}
	parts = append(parts, "[출처: 기상청_지상(종관,ASOS) 시간자료 조회서비스]")
	return strings.Join(parts, ", "), true
}
