// Package handlers provides HTTP request handlers.
// This file contains the streaming chat handler for the ChatHandler.
package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/pkg/sanitize"
	"llm-router-platform/pkg/tokencount"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	streamFinalizeTimeout  = 10 * time.Second
	asyncCacheStoreTimeout = 5 * time.Second
)

func streamFinalizeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), streamFinalizeTimeout)
}

func asyncCacheContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), asyncCacheStoreTimeout)
}

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
				writeOpenAIStreamError(w, chunk.Error)
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

	h.finalizeStream(c.Request.Context(), req, selectedProvider, projectObj, userAPIKey, start, conversationID, originalMessages, logID, promptHash, promptEmbedding, fullText, promptTokens, completionTokens, streamErr, gen)
}

func (h *ChatHandler) finalizeStream(ctx context.Context, req *provider.ChatRequest, selectedProvider *models.Provider, projectObj *models.Project, userAPIKey *models.APIKey, start time.Time, conversationID string, originalMessages []MessageRequest, logID uuid.UUID, promptHash string, promptEmbedding []float32, fullText string, promptTokens int, completionTokens int, streamErr error, gen observability.Generation) {
	// If the upstream provider didn't return a usage block in the stream
	// (chunk.Usage was nil throughout), fall back to local tokenization. This
	// estimate drives BILLING — record the fallback so a sustained spike is
	// observable in metrics.
	usedLocalEstimate := false
	if promptTokens == 0 && completionTokens == 0 && fullText != "" {
		usedLocalEstimate = true
		completionTokens = tokencount.CountTokens(fullText, req.Model)
		for _, m := range req.Messages {
			promptTokens += tokencount.CountTokens(m.Content.Text, req.Model)
		}
	}
	if usedLocalEstimate {
		middleware.LocalTokenFallbackTotal.WithLabelValues(selectedProvider.Name, req.Model).Inc()
		h.logger.Warn("streaming response billed using locally estimated tokens",
			zap.String("provider", selectedProvider.Name),
			zap.String("model", req.Model),
			zap.Int("prompt_tokens", promptTokens),
			zap.Int("completion_tokens", completionTokens),
		)
	}

	gen.End(fullText, promptTokens, completionTokens)

	// Status code semantics:
	//   - clean stream completion: 200 (counted as success)
	//   - upstream/network error mid-stream: 502 (counted as failure)
	//
	// We previously set 206 here, but UsageLog.IsSuccess uses statusCode in
	// [200, 300), so 206 was being counted as success. Anything that reaches
	// this branch with streamErr != nil is *not* a successful generation —
	// either the connection broke or the upstream returned an error chunk.
	statusCode := http.StatusOK
	errStr := ""
	if streamErr != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			// Client disconnected (or request was canceled). Use 499
			// (nginx-style "Client Closed Request") so it doesn't count as
			// a successful response but is distinguishable from upstream 5xx.
			statusCode = 499
		} else {
			statusCode = http.StatusBadGateway
		}
		errStr = sanitize.TruncateErrorMessage(streamErr.Error())
	}

	billingCtx, cancel := streamFinalizeContext(ctx)
	defer cancel()
	if err := h.billing.UpdateUsageTokensAndDeduct(billingCtx, logID, promptTokens, completionTokens, statusCode, time.Since(start).Milliseconds(), errStr, h.balance, userAPIKey.UserID, "LLM Stream: "+req.Model); err != nil {
		h.logger.Warn("billing update failed after stream", zap.Error(err))
	}

	if conversationID != "" && h.memory != nil {
		for _, m := range originalMessages {
			_ = h.memory.AddMessage(ctx, projectObj.ID, &userAPIKey.ID, conversationID, m.Role, m.Content.Text, 0)
		}
		_ = h.memory.AddMessage(ctx, projectObj.ID, &userAPIKey.ID, conversationID, "assistant", fullText, completionTokens)
	}

	if h.cache != nil && promptHash != "" && fullText != "" {
		// Cache store runs after HTTP response is sent, with a small bounded timeout.
		go h.storeInCache(ctx, promptHash, promptEmbedding, fullText, selectedProvider.Name, req.Model, promptTokens, completionTokens) // #nosec G118 -- fire-and-forget cache write after response
	}
}

func writeOpenAIStreamError(w io.Writer, err error) {
	payload := map[string]any{
		"error": map[string]string{
			"message": sanitize.TruncateErrorMessage(err.Error()),
			"type":    "server_error",
			"code":    "upstream_stream_error",
		},
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return
	}
	_, _ = w.Write([]byte("data: "))
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n\ndata: [DONE]\n\n"))
}

func (h *ChatHandler) storeInCache(ctx context.Context, hash string, emb []float32, text string, pid string, m string, promptTokens int, completionTokens int) {
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
	cacheCtx, cancel := asyncCacheContext(ctx)
	defer cancel()
	_ = h.cache.StoreCache(cacheCtx, hash, emb, cachedResp, pid, m, nil)
}
