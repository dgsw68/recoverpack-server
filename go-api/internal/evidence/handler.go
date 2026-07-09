package evidence

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"recoverpack-server/go-api/internal/ai"
	"recoverpack-server/go-api/internal/firebase"
)

type UpdateEvidenceRequest struct {
	Category string `json:"category"`
	Caption  string `json:"caption"`
}

// AnalyzeProjectHandler pulls all project files, triggers the Python AI analysis,
// saves the resulting captions & categories, and returns the list of evidence.
func AnalyzeProjectHandler(fbClient *firebase.Client, aiClient *ai.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		// 1. Verify project exists
		_, err := fbClient.GetProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		}

		// 2. Fetch all registered files for this project
		files, err := fbClient.GetFilesByProject(ctx, projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch project files: " + err.Error()})
			return
		}

		if len(files) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No files registered yet. Please upload files first."})
			return
		}

		// 3. Trigger image classification & caption generation on ai-service
		evidenceList, err := aiClient.AnalyzeFiles(ctx, projectID, files)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI image analysis failed: " + err.Error()})
			return
		}

		// 4. Save evidence to database
		if err := fbClient.SaveEvidence(ctx, evidenceList); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save analyzed evidence: " + err.Error()})
			return
		}

		// 5. Generate high-level project description text (runs asynchronously or synchronously)
		// For robustness in a MVP, run synchronously and save to project
		if overallDesc, err := aiClient.GenerateDescription(ctx, projectID, evidenceList); err == nil {
			_ = fbClient.UpdateProjectDescription(ctx, projectID, overallDesc)
		}

		c.JSON(http.StatusOK, evidenceList)
	}
}

// GetEvidenceHandler returns the evidence list for a project
func GetEvidenceHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		if projectID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID is required"})
			return
		}

		evidenceList, err := fbClient.GetEvidenceByProject(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch evidence: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, evidenceList)
	}
}

// UpdateEvidenceHandler updates category and caption fields (User override)
func UpdateEvidenceHandler(fbClient *firebase.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("projectId")
		evidenceID := c.Param("evidenceId")

		if projectID == "" || evidenceID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Project ID and Evidence ID are required"})
			return
		}

		var req UpdateEvidenceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		updatedItem, err := fbClient.UpdateEvidence(c.Request.Context(), projectID, evidenceID, req.Category, req.Caption)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update evidence: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, updatedItem)
	}
}
