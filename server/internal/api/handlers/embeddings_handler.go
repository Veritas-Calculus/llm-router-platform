// Package handlers provides HTTP request handlers.
// This file contains the embeddings handler for the ChatHandler.
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Embeddings handles embedding generation requests.
func (h *ChatHandler) Embeddings(c *gin.Context) {
	var req EmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	providerReq := &provider.EmbeddingRequest{
		Model:          req.Model,
		Input:          req.Input,
		EncodingFormat: req.EncodingFormat,
	}

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "embeddings", userObj.ID.String(), "", map[string]interface{}{
		"model":           req.Model,
		"encoding_format": req.EncodingFormat,
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
	var resp *provider.EmbeddingResponse
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
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
			"encoding_format": req.EncodingFormat,
		}, req.Input)

		resp, lastErr = client.Embeddings(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
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
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
				"encoding_format": req.EncodingFormat,
			}, req.Input)

			resp, err = client.Embeddings(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
				lastErr = err
				h.logger.Warn("embeddings request failed, trying next API key",
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
				gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
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
		ModelName:  req.Model,
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
			c.JSON(http.StatusNotImplemented, gin.H{"error": "embeddings not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	usageLog.RequestTokens = resp.Usage.PromptTokens
	usageLog.ResponseTokens = resp.Usage.CompletionTokens
	usageLog.TotalTokens = resp.Usage.TotalTokens
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	c.JSON(http.StatusOK, resp)
}
