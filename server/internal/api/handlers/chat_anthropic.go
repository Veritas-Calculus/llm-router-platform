// Package handlers — chat_anthropic.go isolates the Anthropic-compatible
// /v1/messages handler (request + response shapes, streaming SSE encoder)
// from the OpenAI-compatible chat handler so neither grows in lockstep with
// the other. Splits out of the previously 1077-line chat_handler.go per the
// audit "大文件拆分" item.
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AnthropicMessagesRequest represents an Anthropic messages request.
type AnthropicMessagesRequest struct {
	Model       string             `json:"model" binding:"required"`
	Messages    []AnthropicMessage `json:"messages" binding:"required"`
	MaxTokens   int                `json:"max_tokens" binding:"required"`
	Temperature *float64           `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []AnthropicTool    `json:"tools,omitempty"`
}

type AnthropicMessage struct {
	Role    string      `json:"role" binding:"required"`
	Content interface{} `json:"content" binding:"required"`
}

type AnthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// AnthropicMessages handles Anthropic-compatible message requests.
func (h *ChatHandler) AnthropicMessages(c *gin.Context) {
	var anthroReq AnthropicMessagesRequest
	if err := c.ShouldBindJSON(&anthroReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Map Anthropic request to internal ChatRequest
	internalMessages := mapAnthropicMessages(anthroReq)

	var temp float64
	if anthroReq.Temperature != nil {
		temp = *anthroReq.Temperature
	}

	providerReq := &provider.ChatRequest{
		Model:       anthroReq.Model,
		Messages:    internalMessages,
		MaxTokens:   anthroReq.MaxTokens,
		Temperature: temp,
		Stream:      anthroReq.Stream,
	}

	// Routing and quota check logic (simplified for brevity, reuses internal logic)
	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	if done := h.applyDLP(c, projectObj, internalMessages); done {
		return
	}

	selectedProvider, apiKey, err := h.router.RouteForAPIKey(c.Request.Context(), anthroReq.Model, userAPIKey)
	if err != nil {
		if writeAPIKeyPolicyError(c, err) {
			return
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no providers available"})
		return
	}
	c.Set("llm_model", anthroReq.Model)
	c.Set("provider_name", selectedProvider.Name)
	c.Set("provider_id", selectedProvider.ID.String())

	trace := h.startRequestTrace(c, "anthropic_messages", projectObj.ID.String(), "", map[string]interface{}{
		"model":  anthroReq.Model,
		"stream": anthroReq.Stream,
	})
	defer trace.End()

	if quotaErr := h.checkProjectQuota(c, projectObj, userAPIKey); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": *quotaErr})
		return
	}

	start := time.Now()

	// Handle streaming via existing infrastructure
	if anthroReq.Stream {
		h.handleAnthropicStream(c, anthroReq, providerReq, selectedProvider, apiKey, userAPIKey, projectObj, start, trace)
		return
	}

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, anthroReq.Model, map[string]interface{}{
		"temperature": providerReq.Temperature,
		"max_tokens":  providerReq.MaxTokens,
	}, providerReq.Messages)

	chatCtx := router.WithIterationGate(c.Request.Context(), h.buildIterationGate(c, projectObj, userAPIKey))
	result, err := h.router.ExecuteChat(chatCtx, selectedProvider, apiKey, providerReq, 3)
	if err != nil || result == nil {
		if err == nil {
			err = errors.New("empty provider response")
		}
		gen.EndWithError(err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider error"})
		return
	}

	resp := result.Response
	if len(resp.Choices) == 0 {
		err := errors.New("empty provider choices")
		gen.EndWithError(err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider error"})
		return
	}
	responseText := resp.Choices[0].Message.Content.Text
	gen.End(responseText, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
	latency := time.Since(start)

	// Convert back to Anthropic response format
	anthroResp := gin.H{
		"id":    resp.ID,
		"type":  "message",
		"role":  "assistant",
		"model": resp.Model,
		"content": []gin.H{
			{
				"type": "text",
				"text": responseText,
			},
		},
		"usage": gin.H{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		},
	}

	// Record usage
	usageLog := &models.UsageLog{
		UserID:         userAPIKey.UserID,
		ProjectID:      projectObj.ID,
		Channel:        userAPIKey.Channel,
		APIKeyID:       userAPIKey.ID,
		ProviderID:     selectedProvider.ID,
		ModelName:      anthroReq.Model,
		Latency:        latency.Milliseconds(),
		StatusCode:     http.StatusOK,
		RequestTokens:  resp.Usage.PromptTokens,
		ResponseTokens: resp.Usage.CompletionTokens,
		TotalTokens:    resp.Usage.TotalTokens,
	}
	if err := h.billing.RecordUsageAndDeduct(c.Request.Context(), usageLog, h.balance, userAPIKey.UserID, "Anthropic API: "+anthroReq.Model); err != nil {
		h.logger.Warn("billing deduction failed", zap.Error(err), zap.String("model", sanitize.LogValue(anthroReq.Model)))
	}

	c.JSON(http.StatusOK, anthroResp)
}

// mapAnthropicMessages converts Anthropic message format to internal provider.Message format.
func mapAnthropicMessages(anthroReq AnthropicMessagesRequest) []provider.Message {
	messages := make([]provider.Message, 0)

	// Add system message if present
	if anthroReq.System != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: provider.StringContent(anthroReq.System),
		})
	}

	for _, m := range anthroReq.Messages {
		content := ""
		switch v := m.Content.(type) {
		case string:
			content = v
		case []interface{}:
			data, _ := json.Marshal(v)
			content = string(data)
		}
		messages = append(messages, provider.Message{
			Role:    m.Role,
			Content: provider.StringContent(content),
		})
	}
	return messages
}

// handleAnthropicStream handles the streaming path for Anthropic-compatible requests.
func (h *ChatHandler) handleAnthropicStream(c *gin.Context, anthroReq AnthropicMessagesRequest, providerReq *provider.ChatRequest, selectedProvider *models.Provider, apiKey *models.ProviderAPIKey, userAPIKey *models.APIKey, projectObj *models.Project, start time.Time, trace observability.Trace) {
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:  projectObj.ID,
		Channel:    userAPIKey.Channel,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  anthroReq.Model,
		Latency:    0,
		StatusCode: http.StatusProcessing,
	}
	if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
		h.logger.Warn("billing pre-record failed", zap.Error(err), zap.String("model", sanitize.LogValue(anthroReq.Model)))
	}

	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, anthroReq.Model, map[string]interface{}{
		"temperature": providerReq.Temperature,
		"max_tokens":  providerReq.MaxTokens,
		"stream":      true,
	}, providerReq.Messages)

	streamResult, err := h.router.ExecuteStreamChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)
	if err != nil {
		gen.EndWithError(err)
		h.logger.Error("anthropic stream failed", zap.Error(err))
		if billingErr := h.billing.UpdateUsageTokens(c.Request.Context(), usageLog.ID, 0, 0, http.StatusBadGateway, time.Since(start).Milliseconds(), err.Error()); billingErr != nil {
			h.logger.Warn("billing update failed", zap.Error(billingErr))
		}
		c.JSON(http.StatusBadGateway, gin.H{"type": "error", "error": gin.H{"type": "api_error", "message": "upstream stream failed"}})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// Anthropic message_start event
	msgStartEvent := gin.H{
		"type": "message_start",
		"message": gin.H{
			"id":      "msg_" + uuid.New().String()[:8],
			"type":    "message",
			"role":    "assistant",
			"model":   anthroReq.Model,
			"content": []interface{}{},
			"usage":   gin.H{"input_tokens": 0, "output_tokens": 0},
		},
	}
	data, _ := json.Marshal(msgStartEvent)
	_, _ = c.Writer.Write([]byte("event: message_start\ndata: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))

	// content_block_start
	blockStart := gin.H{"type": "content_block_start", "index": 0, "content_block": gin.H{"type": "text", "text": ""}}
	data, _ = json.Marshal(blockStart)
	_, _ = c.Writer.Write([]byte("event: content_block_start\ndata: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()

	var totalOutput int
	var fullText string
	var streamErr error
	statusCode := http.StatusOK
	for {
		select {
		case <-c.Request.Context().Done():
			streamErr = c.Request.Context().Err()
			statusCode = 499
		case chunk, ok := <-streamResult.Stream:
			if !ok {
				gen.End(fullText, 0, totalOutput)
				goto complete
			}
			if chunk.Error != nil {
				streamErr = chunk.Error
				statusCode = http.StatusBadGateway
				gen.EndWithError(chunk.Error)
				goto complete
			}
			if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
				continue
			}

			totalOutput++
			fullText += chunk.Choices[0].Delta.Content
			delta := gin.H{
				"type":  "content_block_delta",
				"index": 0,
				"delta": gin.H{"type": "text_delta", "text": chunk.Choices[0].Delta.Content},
			}
			data, _ = json.Marshal(delta)
			_, _ = c.Writer.Write([]byte("event: content_block_delta\ndata: "))
			_, _ = c.Writer.Write(data)
			_, _ = c.Writer.Write([]byte("\n\n"))
			c.Writer.Flush()
		}
		if streamErr != nil {
			gen.EndWithError(streamErr)
			goto complete
		}
	}

complete:
	if streamErr != nil {
		latency := time.Since(start)
		billingCtx, cancel := streamFinalizeContext(c.Request.Context())
		defer cancel()
		if err := h.billing.UpdateUsageTokensAndDeduct(billingCtx, usageLog.ID, 0, totalOutput, statusCode, latency.Milliseconds(), sanitize.TruncateErrorMessage(streamErr.Error()), h.balance, userAPIKey.UserID, "Anthropic stream: "+anthroReq.Model); err != nil {
			h.logger.Warn("billing update failed", zap.Error(err))
		}
		return
	}

	// content_block_stop + message_delta + message_stop
	blockStop := gin.H{"type": "content_block_stop", "index": 0}
	data, _ = json.Marshal(blockStop)
	_, _ = c.Writer.Write([]byte("event: content_block_stop\ndata: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))

	msgDelta := gin.H{"type": "message_delta", "delta": gin.H{"stop_reason": "end_turn"}, "usage": gin.H{"output_tokens": totalOutput}}
	data, _ = json.Marshal(msgDelta)
	_, _ = c.Writer.Write([]byte("event: message_delta\ndata: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))

	msgStop := gin.H{"type": "message_stop"}
	data, _ = json.Marshal(msgStop)
	_, _ = c.Writer.Write([]byte("event: message_stop\ndata: "))
	_, _ = c.Writer.Write(data)
	_, _ = c.Writer.Write([]byte("\n\n"))
	c.Writer.Flush()

	latency := time.Since(start)
	billingCtx, cancel := streamFinalizeContext(c.Request.Context())
	defer cancel()
	if err := h.billing.UpdateUsageTokensAndDeduct(billingCtx, usageLog.ID, 0, totalOutput, http.StatusOK, latency.Milliseconds(), "", h.balance, userAPIKey.UserID, "Anthropic stream: "+anthroReq.Model); err != nil {
		h.logger.Warn("billing update failed", zap.Error(err))
	}
}
