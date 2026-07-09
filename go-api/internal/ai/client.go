package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"recoverpack-server/go-api/internal/models"
)

// Client is the HTTP client for communicating with the internal Python ai-service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new AI service client.
func NewClient() *Client {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 45 * time.Second, // Long timeout for AI generation
		},
	}
}

// --- Request/Response DTOs mirroring FastAPI schemas ---

type FileAnalysisInput struct {
	ID       string `json:"id"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileURL  string `json:"file_url"`
	MimeType string `json:"mime_type"`
}

type AnalyzeImageRequest struct {
	ProjectID string              `json:"project_id"`
	Files     []FileAnalysisInput `json:"files"`
}

type EvidenceAnalysisItem struct {
	FileID  string `json:"file_id"`
	FileURL string `json:"file_url"`
	Category string `json:"category"`
	Caption  string `json:"caption"`
}

type AnalyzeImageResponse struct {
	Evidence []EvidenceAnalysisItem `json:"evidence"`
}

type EvidenceSummaryItem struct {
	FileID   string `json:"file_id"`
	Category string `json:"category"`
	Caption  string `json:"caption"`
}

type GenerateDescriptionRequest struct {
	ProjectID string                `json:"project_id"`
	Evidence  []EvidenceSummaryItem `json:"evidence"`
}

type GenerateDescriptionResponse struct {
	Description string `json:"description"`
}

type GenerateTimelineRequest struct {
	ProjectID string                `json:"project_id"`
	Evidence  []EvidenceSummaryItem `json:"evidence"`
}

type TimelineEventItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	EventDate   string `json:"event_date"`
}

type GenerateTimelineResponse struct {
	Timeline []TimelineEventItem `json:"timeline"`
}

// --- Client Methods ---

// AnalyzeFiles calls the Python service to classify and write captions for files
func (c *Client) AnalyzeFiles(ctx context.Context, projectID string, files []models.ProjectFile) ([]models.Evidence, error) {
	url := fmt.Sprintf("%s/internal/analyze-image", c.baseURL)

	inputFiles := make([]FileAnalysisInput, len(files))
	for i, f := range files {
		inputFiles[i] = FileAnalysisInput{
			ID:       f.ID,
			FileName: f.FileName,
			FileType: f.FileType,
			FileURL:  f.FileURL,
			MimeType: f.MimeType,
		}
	}

	reqPayload := AnalyzeImageRequest{
		ProjectID: projectID,
		Files:     inputFiles,
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analyze request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai-service call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai-service returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respPayload AnalyzeImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return nil, fmt.Errorf("failed to decode analyze response: %w", err)
	}

	now := time.Now()
	evidenceList := make([]models.Evidence, len(respPayload.Evidence))
	for i, item := range respPayload.Evidence {
		evidenceList[i] = models.Evidence{
			ID:        fmt.Sprintf("ev_%s", item.FileID), // Generate unique evidence ID
			ProjectID: projectID,
			FileID:    item.FileID,
			FileURL:   item.FileURL,
			Category:  item.Category,
			Caption:   item.Caption,
			CreatedAt: now,
		}
	}

	return evidenceList, nil
}

// GenerateDescription calls the Python service to create an overall summary paragraph
func (c *Client) GenerateDescription(ctx context.Context, projectID string, evidence []models.Evidence) (string, error) {
	url := fmt.Sprintf("%s/internal/generate-description", c.baseURL)

	summaryItems := make([]EvidenceSummaryItem, len(evidence))
	for i, ev := range evidence {
		summaryItems[i] = EvidenceSummaryItem{
			FileID:   ev.FileID,
			Category: ev.Category,
			Caption:  ev.Caption,
		}
	}

	reqPayload := GenerateDescriptionRequest{
		ProjectID: projectID,
		Evidence:  summaryItems,
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal description request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai-service description call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("ai-service description returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respPayload GenerateDescriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return "", fmt.Errorf("failed to decode description response: %w", err)
	}

	return respPayload.Description, nil
}

// GenerateTimeline calls the Python service to generate a chronological event list
func (c *Client) GenerateTimeline(ctx context.Context, projectID string, evidence []models.Evidence) ([]models.TimelineEvent, error) {
	url := fmt.Sprintf("%s/internal/generate-timeline", c.baseURL)

	summaryItems := make([]EvidenceSummaryItem, len(evidence))
	for i, ev := range evidence {
		summaryItems[i] = EvidenceSummaryItem{
			FileID:   ev.FileID,
			Category: ev.Category,
			Caption:  ev.Caption,
		}
	}

	reqPayload := GenerateTimelineRequest{
		ProjectID: projectID,
		Evidence:  summaryItems,
	}

	jsonBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal timeline request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai-service timeline call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ai-service timeline returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respPayload GenerateTimelineResponse
	if err := json.NewDecoder(resp.Body).Decode(&respPayload); err != nil {
		return nil, fmt.Errorf("failed to decode timeline response: %w", err)
	}

	now := time.Now()
	timelineEvents := make([]models.TimelineEvent, len(respPayload.Timeline))
	for i, item := range respPayload.Timeline {
		timelineEvents[i] = models.TimelineEvent{
			ID:          fmt.Sprintf("time_%d_%s", now.UnixNano(), item.Title),
			ProjectID:   projectID,
			Title:       item.Title,
			Description: item.Description,
			EventDate:   item.EventDate,
			CreatedAt:   now,
		}
	}

	return timelineEvents, nil
}
