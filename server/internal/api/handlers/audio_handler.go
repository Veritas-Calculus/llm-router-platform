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

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	// Observability: Start Trace
	trace := h.startRequestTrace(c, "transcribe_audio", projectObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
		"filename":        providerReq.FileName,
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
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
	}, providerReq.Prompt)

	result, err := h.router.ExecuteAudio(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		h.handleProviderError(c, err, start, userAPIKey, projectObj, selectedProvider, model)
		return
	}

	resp := result.Response
	gen.End(resp.Text, 0, 0)

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:         userAPIKey.UserID,
		ProjectID:      projectObj.ID,
		Channel:        userAPIKey.Channel,
		APIKeyID:       userAPIKey.ID,
		ProviderID:     selectedProvider.ID,
		ModelName:      model,
		BytesProcessed: int64(len(fileBytes)),
		Latency:        latency.Milliseconds(),
		StatusCode:     http.StatusOK,
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
