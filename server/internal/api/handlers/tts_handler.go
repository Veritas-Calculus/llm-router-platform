// Package handlers provides HTTP request handlers.
// This file contains the text-to-speech handler for the ChatHandler.
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

// SpeechSynthesisRequest represents a text-to-speech request from the user.
type SpeechSynthesisRequest struct {
	Model          string  `json:"model" binding:"required"`
	Input          string  `json:"input" binding:"required"`
	Voice          string  `json:"voice" binding:"required"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// SynthesizeSpeech handles text-to-speech synthesis requests.
func (h *ChatHandler) SynthesizeSpeech(c *gin.Context) {
	var req SpeechSynthesisRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	selectedProvider, apiKey, err := h.router.RouteForAPIKey(c.Request.Context(), req.Model, userAPIKey)
	if err != nil {
		if writeAPIKeyPolicyError(c, err) {
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers for model: " + req.Model})
		return
	}
	c.Set("llm_model", req.Model)
	c.Set("provider_name", selectedProvider.Name)
	c.Set("provider_id", selectedProvider.ID.String())

	providerReq := &provider.SpeechRequest{
		Model:          req.Model,
		Input:          req.Input,
		Voice:          req.Voice,
		ResponseFormat: req.ResponseFormat,
		Speed:          req.Speed,
	}

	// Observability: Start Trace
	trace := h.startRequestTrace(c, "synthesize_speech", projectObj.ID.String(), "", map[string]interface{}{
		"model":           req.Model,
		"voice":           req.Voice,
		"response_format": req.ResponseFormat,
		"speed":           req.Speed,
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

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"voice":           req.Voice,
		"response_format": req.ResponseFormat,
		"speed":           req.Speed,
	}, providerReq.Input)

	result, err := h.router.ExecuteSpeech(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:     userAPIKey.UserID,
			ProjectID:  projectObj.ID,
			Channel:    userAPIKey.Channel,
			APIKeyID:   userAPIKey.ID,
			ProviderID: selectedProvider.ID,
			ModelName:  req.Model,
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
			c.JSON(http.StatusNotImplemented, gin.H{"error": "speech synthesis not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	gen.End("Speech synthesized successfully", 0, 0)

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:  projectObj.ID,
		Channel:    userAPIKey.Channel,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  req.Model,
		Latency:    latency.Milliseconds(),
		StatusCode: http.StatusOK,
	}
	if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
		h.logger.Warn("billing record failed", zap.Error(err))
	}

	// Return raw audio binary data
	c.Data(http.StatusOK, result.Response.ContentType, result.Response.Audio)
}
