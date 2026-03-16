// Package handlers provides HTTP request handlers.
// This file contains the streaming chat handler for the ChatHandler.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"

	"github.com/gin-gonic/gin"
)

// handleStreamingChat handles streaming chat completion requests.
// It receives a pre-established stream channel (connection already opened with retry by Router).
func (h *ChatHandler) handleStreamingChat(c *gin.Context, chunks <-chan provider.StreamChunk, req *provider.ChatRequest, selectedProvider *models.Provider, userObj *models.User, userAPIKey *models.APIKey, start time.Time, trace observability.Trace, conversationID string, originalMessages []MessageRequest) {
	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}, req.Messages)

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	var fullText string
	var promptTokens, completionTokens, totalTokens int

	c.Stream(func(w io.Writer) bool {
		chunk, ok := <-chunks
		if !ok {
			return false
		}

		if chunk.Error != nil {
			gen.EndWithError(chunk.Error)
			return false
		}

		if chunk.Done {
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return false
		}

		if len(chunk.Choices) > 0 {
			fullText += chunk.Choices[0].Delta.Content
		}

		if chunk.Usage != nil {
			promptTokens = chunk.Usage.PromptTokens
			completionTokens = chunk.Usage.CompletionTokens
			totalTokens = chunk.Usage.TotalTokens
		}

		data, err := json.Marshal(chunk)
		if err != nil {
			return false
		}

		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(data)
		_, _ = w.Write([]byte("\n\n"))
		return true
	})

	// Record usage after streaming completes
	gen.End(fullText, promptTokens, completionTokens)

	if conversationID != "" && h.memory != nil {
		for _, m := range originalMessages {
			_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, conversationID, m.Role, m.Content.Text, 0)
		}
		_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, conversationID, "assistant", fullText, completionTokens)
	}

	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:         userObj.ID,
		APIKeyID:       userAPIKey.ID,
		ProviderID:     selectedProvider.ID,
		ModelName:      req.Model,
		RequestTokens:  promptTokens,
		ResponseTokens: completionTokens,
		TotalTokens:    totalTokens,
		Latency:        latency.Milliseconds(),
		StatusCode:     http.StatusOK,
	}
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)
}
