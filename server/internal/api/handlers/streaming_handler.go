// Package handlers provides HTTP request handlers.
// This file contains the streaming chat handler for the ChatHandler.
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// handleStreamingChat handles streaming chat completion requests.
// It receives a pre-established stream channel (connection already opened with retry by Router).
func (h *ChatHandler) handleStreamingChat(c *gin.Context, chunks <-chan provider.StreamChunk, req *provider.ChatRequest, selectedProvider *models.Provider, projectObj *models.Project, userAPIKey *models.APIKey, start time.Time, trace observability.Trace, conversationID string, originalMessages []MessageRequest, logID uuid.UUID, promptHash string, promptEmbedding []float32) {
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
	var promptTokens, completionTokens int
	var streamErr error

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			streamErr = c.Request.Context().Err()
			return false
		case chunk, ok := <-chunks:
			if !ok {
				return false
			}

			if chunk.Error != nil {
				streamErr = chunk.Error
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
			}

			data, err := json.Marshal(chunk)
			if err != nil {
				return false
			}

			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			return true
		}
	})

	// Record usage after streaming completes (success or partial failure)
	// If provider didn't return usage in stream chunks, estimate tokens from text
	if promptTokens == 0 && completionTokens == 0 && fullText != "" {
		// Estimate output tokens from accumulated text (~4 chars per token)
		completionTokens = estimateTokenCount(fullText)
		// Estimate input tokens from request messages
		for _, m := range req.Messages {
			promptTokens += estimateTokenCount(m.Content.Text)
		}
	}
	gen.End(fullText, promptTokens, completionTokens)

	statusCode := http.StatusOK
	errStr := ""
	if streamErr != nil {
		statusCode = http.StatusPartialContent
		errStr = streamErr.Error()
	}

	_ = h.billing.UpdateUsageTokens(context.Background(), logID, promptTokens, completionTokens, statusCode, time.Since(start).Milliseconds(), errStr)

	if conversationID != "" && h.memory != nil {
		for _, m := range originalMessages {
			_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, conversationID, m.Role, m.Content.Text, 0)
		}
		_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, conversationID, "assistant", fullText, completionTokens)
	}

	if h.cache != nil && promptHash != "" && fullText != "" {
		go func(hash string, emb []float32, text string, pid string, m string) {
			if len(emb) == 0 {
				emb = make([]float32, 1536)
			}
			cachedResp := provider.ChatResponse{
				ID:    uuid.New().String(),
				Model: m,
				Choices: []provider.Choice{
					{
						Message: provider.Message{
							Role:    "assistant",
							Content: provider.FlexibleContent{Text: text},
						},
					},
				},
				Usage: provider.Usage{
					PromptTokens:     promptTokens,
					CompletionTokens: completionTokens,
					TotalTokens:      promptTokens + completionTokens,
				},
			}
			_ = h.cache.StoreCache(context.Background(), hash, emb, cachedResp, pid, m, nil)
		}(promptHash, promptEmbedding, fullText, selectedProvider.Name, req.Model)
	}
}

// estimateTokenCount estimates token count from text.
// Uses a heuristic of ~4 characters per token for English (GPT-family tokenizers).
// For CJK text, each character is roughly 1-2 tokens, so we use a conservative
// estimate that works reasonably for mixed-language content.
func estimateTokenCount(text string) int {
	if text == "" {
		return 0
	}
	charCount := len(text)
	// ~4 bytes per token is a reasonable average for mixed content
	tokens := (charCount + 3) / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}
