// Package handlers provides HTTP request handlers.
// This file contains the image generation handler for the ChatHandler.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GenerateImage handles image generation requests.
func (h *ChatHandler) GenerateImage(c *gin.Context) {
	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Model might be missing if prompt provided directly, default to dall-e-3
	model := req.Model
	if model == "" {
		model = "dall-e-3"
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	providerReq := &provider.ImageGenerationRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              req.N,
		Size:           req.Size,
		ResponseFormat: req.ResponseFormat,
	}

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "generate_image", projectObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"size":            req.Size,
		"response_format": req.ResponseFormat,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

	if quotaErr := h.checkProjectQuota(c, projectObj); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{
				"message": *quotaErr,
				"type":    "quota_exceeded",
				"code":    "quota_exceeded",
			},
		})
		return
	}

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, model, map[string]interface{}{
		"size":            req.Size,
		"response_format": req.ResponseFormat,
		"n":               req.N,
	}, req.Prompt)

	result, err := h.router.ExecuteImage(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:     userAPIKey.UserID,
			ProjectID:   projectObj.ID,
			APIKeyID:   userAPIKey.ID,
			ProviderID: selectedProvider.ID,
			ModelName:  model,
			Latency:    latency.Milliseconds(),
			StatusCode: http.StatusBadGateway,
		}
		if err != nil {
			usageLog.ErrorMessage = err.Error()
			if err == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		if err == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "image generation not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	gen.End("Image generated successfully", 0, 0)

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:   projectObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  model,
		Latency:    latency.Milliseconds(),
		StatusCode: http.StatusOK,
	}
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	c.JSON(http.StatusOK, result.Response)
}
