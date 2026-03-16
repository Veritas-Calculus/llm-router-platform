// Package handlers provides HTTP request handlers.
// This file contains the embeddings handler for the ChatHandler.
package handlers

import (
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"encoding_format": req.EncodingFormat,
	}, req.Input)

	result, err := h.router.ExecuteEmbeddings(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:     userObj.ID,
			APIKeyID:   userAPIKey.ID,
			ProviderID: selectedProvider.ID,
			ModelName:  req.Model,
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
			c.JSON(http.StatusNotImplemented, gin.H{"error": "embeddings not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	resp := result.Response
	gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:         userObj.ID,
		APIKeyID:       userAPIKey.ID,
		ProviderID:     selectedProvider.ID,
		ModelName:      req.Model,
		Latency:        latency.Milliseconds(),
		StatusCode:     http.StatusOK,
		RequestTokens:  resp.Usage.PromptTokens,
		ResponseTokens: resp.Usage.CompletionTokens,
		TotalTokens:    resp.Usage.TotalTokens,
	}
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	c.JSON(http.StatusOK, resp)
}
