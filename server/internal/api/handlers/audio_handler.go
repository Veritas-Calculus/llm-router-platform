// Package handlers provides HTTP request handlers.
// This file contains the audio transcription handler for the ChatHandler.
package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"github.com/google/uuid"
)

// TranscribeAudio handles audio transcription requests.
func (h *ChatHandler) TranscribeAudio(c *gin.Context) {
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required: " + err.Error()})
		return
	}
	defer func() { _ = file.Close() }()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	model := c.PostForm("model")
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	// Read optional fields
	var temperature float64
	tempStr := c.PostForm("temperature")
	if tempStr != "" {
		_, _ = fmt.Sscanf(tempStr, "%f", &temperature)
	}

	providerReq := &provider.AudioTranscriptionRequest{
		File:           fileBytes,
		FileName:       fileHeader.Filename,
		Model:          model,
		Language:       c.PostForm("language"),
		Prompt:         c.PostForm("prompt"),
		ResponseFormat: c.PostForm("response_format"),
		Temperature:    temperature,
	}

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "transcribe_audio", projectObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
		"filename":        providerReq.FileName,
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
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
	}, providerReq.Prompt)

	result, err := h.router.ExecuteAudio(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

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
			c.JSON(http.StatusNotImplemented, gin.H{"error": "audio transcription not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	resp := result.Response
	gen.End(resp.Text, 0, 0)

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
	if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
		h.logger.Warn("billing record failed", zap.Error(err))
	}

	// In OpenAI's API, the text format requests return plain text string directly.
	// The client provider wrapper handles format translation into the unified struct.
	if providerReq.ResponseFormat == "text" || providerReq.ResponseFormat == "srt" || providerReq.ResponseFormat == "vtt" {
		c.String(http.StatusOK, resp.Text)
		return
	}

	c.JSON(http.StatusOK, resp)
}
