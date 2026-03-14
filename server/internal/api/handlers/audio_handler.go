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
	"github.com/google/uuid"
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

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "transcribe_audio", userObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
		"filename":        providerReq.FileName,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

	if quotaErr := h.checkUserQuota(c, userObj); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": gin.H{
				"message": *quotaErr,
				"type":    "quota_exceeded",
				"code":    "quota_exceeded",
			},
		})
		return
	}

	maxRetries := 3
	var resp *provider.AudioTranscriptionResponse
	var lastErr error
	currentAPIKey := apiKey

	if !selectedProvider.RequiresAPIKey {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, nil)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}
		genName := "Provider: " + selectedProvider.Name
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
			"language":        providerReq.Language,
			"response_format": providerReq.ResponseFormat,
			"temperature":     providerReq.Temperature,
		}, providerReq.Prompt)

		resp, lastErr = client.TranscribeAudio(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			gen.End(resp.Text, 0, 0)
		}
	} else {
		for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
			client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
			if err != nil {
				h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
				lastErr = err
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			genName := "Provider: " + selectedProvider.Name
			if attempt > 0 {
				genName += fmt.Sprintf(" (Retry %d)", attempt)
			}
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
				"language":        providerReq.Language,
				"response_format": providerReq.ResponseFormat,
				"temperature":     providerReq.Temperature,
			}, providerReq.Prompt)

			resp, err = client.TranscribeAudio(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
				lastErr = err
				h.logger.Warn("audio transcription request failed, trying next API key",
					zap.Error(err),
					zap.Int("attempt", attempt+1),
					zap.String("key_prefix", currentAPIKey.KeyPrefix),
				)

				errStr := err.Error()
				if isQuotaOrRateLimitError(errStr) {
					h.router.MarkKeyFailed(currentAPIKey.ID, errStr)
				}
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}
			if resp != nil {
				gen.End(resp.Text, 0, 0)
			}
			h.router.ClearKeyFailure(currentAPIKey.ID)
			break
		}
	}

	latency := time.Since(start)

	usageLog := &models.UsageLog{
		UserID:     userObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  model,
		Latency:    latency.Milliseconds(),
	}

	if resp == nil {
		usageLog.StatusCode = http.StatusBadGateway
		if lastErr != nil {
			if lastErr == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
			usageLog.ErrorMessage = lastErr.Error()
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		if lastErr == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "audio transcription not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	// In OpenAI's API, the text format requests return plain text string directly.
	// The client provider wrapper handles format translation into the unified struct.
	if providerReq.ResponseFormat == "text" || providerReq.ResponseFormat == "srt" || providerReq.ResponseFormat == "vtt" {
		c.String(http.StatusOK, resp.Text)
		return
	}

	c.JSON(http.StatusOK, resp)
}
