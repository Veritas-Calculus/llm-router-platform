// Package handlers provides HTTP request handlers.
// This file contains the image generation handler for the ChatHandler.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	selectedProvider, apiKey, err := h.router.RouteForAPIKey(c.Request.Context(), model, userAPIKey)
	if err != nil {
		if writeAPIKeyPolicyError(c, err) {
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}
	c.Set("llm_model", model)
	c.Set("provider_name", selectedProvider.Name)
	c.Set("provider_id", selectedProvider.ID.String())

	providerReq := &provider.ImageGenerationRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              req.N,
		Size:           req.Size,
		ResponseFormat: req.ResponseFormat,
	}

	// Observability: Start Trace
	trace := h.startRequestTrace(c, "generate_image", projectObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"size":            req.Size,
		"response_format": req.ResponseFormat,
	})
	defer trace.End()

	if quotaErr := h.checkProjectQuota(c, projectObj, userAPIKey); quotaErr != nil {
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
	}, providerReq.Prompt)

	result, err := h.router.ExecuteImage(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:     userAPIKey.UserID,
			ProjectID:  projectObj.ID,
			Channel:    userAPIKey.Channel,
			APIKeyID:   userAPIKey.ID,
			ProviderID: selectedProvider.ID,
			ModelName:  model,
			Latency:    latency.Milliseconds(),
			StatusCode: http.StatusBadGateway,
		}
		if err != nil {
			usageLog.ErrorMessage = sanitize.TruncateErrorMessage(err.Error())
			if err == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
			h.logger.Warn("billing record failed", zap.Error(err))
		}

		if err == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "image generation not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	gen.End("Image generated successfully", 0, 0)

	latency := time.Since(start)
	itemCount := len(result.Response.Data)
	if itemCount == 0 {
		itemCount = req.N
	}
	if itemCount <= 0 {
		itemCount = 1
	}
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:  projectObj.ID,
		Channel:    userAPIKey.Channel,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  model,
		ItemCount:  itemCount,
		Latency:    latency.Milliseconds(),
		StatusCode: http.StatusOK,
	}
	if err := h.billing.RecordUsageAndDeduct(c.Request.Context(), usageLog, h.balance, userAPIKey.UserID, "Image generation: "+model); err != nil {
		h.logger.Warn("billing deduction failed", zap.Error(err), zap.String("model", sanitize.LogValue(model)))
	}

	c.JSON(http.StatusOK, result.Response)
}
