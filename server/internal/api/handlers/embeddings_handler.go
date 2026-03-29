// Package handlers provides HTTP request handlers.
// This file contains the embeddings handler for the ChatHandler.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/dlp"
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

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// === Data Loss Prevention (DLP) ===
	if projectObj.DlpConfig != nil && projectObj.DlpConfig.IsEnabled {
		rawBytes, _ := json.Marshal(req.Input)
		rawStr := string(rawBytes)

		switch projectObj.DlpConfig.Strategy {
		case dlp.StrategyBlock:
			if dlp.HasPII(rawStr, projectObj.DlpConfig) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Request blocked by Data Loss Prevention (DLP) policy due to sensitive information."})
				return
			}
		case dlp.StrategyRedact:
			scrubbedStr := dlp.ScrubText(rawStr, projectObj.DlpConfig)
			var newContent json.RawMessage
			if err := json.Unmarshal([]byte(scrubbedStr), &newContent); err == nil {
				providerReq.Input = newContent
			}
		}
	}

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "embeddings", projectObj.ID.String(), "", map[string]interface{}{
		"model":           req.Model,
		"encoding_format": req.EncodingFormat,
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

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"encoding_format": req.EncodingFormat,
	}, req.Input)

	result, err := h.router.ExecuteEmbeddings(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		h.handleProviderError(c, err, start, userAPIKey, projectObj, selectedProvider, req.Model)
		return
	}

	resp := result.Response
	gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:         userAPIKey.UserID,
		ProjectID:      projectObj.ID,
		Channel:        userAPIKey.Channel,
		APIKeyID:       userAPIKey.ID,
		ProviderID:     selectedProvider.ID,
		ModelName:      req.Model,
		Latency:        latency.Milliseconds(),
		StatusCode:     http.StatusOK,
		RequestTokens:  resp.Usage.PromptTokens,
		ResponseTokens: resp.Usage.CompletionTokens,
		TotalTokens:    resp.Usage.TotalTokens,
	}
	if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
		h.logger.Warn("billing record failed", zap.Error(err))
	}

	c.JSON(http.StatusOK, resp)
}
