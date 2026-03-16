// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/pkg/apierror"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)


// ChatHandler handles chat completion endpoints.
type ChatHandler struct {
	router  *router.Router
	billing *billing.Service
	memory  *memory.Service
	obsInfo observability.Service
	logger  *zap.Logger
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(r *router.Router, b *billing.Service, m *memory.Service, obs observability.Service, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		router:  r,
		billing: b,
		memory:  m,
		obsInfo: obs,
		logger:  logger,
	}
}

// checkUserQuota verifies the user hasn't exceeded their monthly quota.
// Returns nil if within quota, or an error message if exceeded.
func (h *ChatHandler) checkUserQuota(c *gin.Context, userObj *models.User) *string {
	// Skip quota check if no limits set (0 = unlimited)
	if userObj.MonthlyTokenLimit == 0 && userObj.MonthlyBudgetUSD == 0 {
		return nil
	}

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	summary, err := h.billing.GetUsageSummary(c.Request.Context(), userObj.ID, monthStart, now)
	if err != nil {
		h.logger.Error("quota check failed", zap.Error(err))
		msg := "unable to verify quota — please try again"
		return &msg // fail-closed: reject if quota cannot be verified
	}

	if userObj.MonthlyTokenLimit > 0 && summary.TotalTokens >= userObj.MonthlyTokenLimit {
		msg := "monthly token quota exceeded"
		return &msg
	}

	if userObj.MonthlyBudgetUSD > 0 && summary.TotalCost >= userObj.MonthlyBudgetUSD {
		msg := "monthly budget quota exceeded"
		return &msg
	}

	return nil
}

// ChatCompletionRequest represents a chat completion request.
type ChatCompletionRequest struct {
	Model              string           `json:"model" binding:"required"`
	Messages           []MessageRequest `json:"messages" binding:"required,min=1"`
	MaxTokens          int              `json:"max_tokens,omitempty"`
	Temperature        float64          `json:"temperature,omitempty"`
	Stream             bool             `json:"stream,omitempty"`
	Tools              json.RawMessage  `json:"tools,omitempty"`
	ToolChoice         json.RawMessage  `json:"tool_choice,omitempty"`
	ConversationID     string           `json:"conversation_id,omitempty"`
	ResumeFromStreamID string           `json:"resume_from_stream_id,omitempty"` // For resuming broken streams
}

// MessageRequest represents a message in the request.
type MessageRequest struct {
	Role    string                  `json:"role" binding:"required"`
	Content provider.FlexibleContent `json:"content" binding:"required"`
}

// EmbeddingsRequest represents an embeddings request from the user.
type EmbeddingsRequest struct {
	Model          string      `json:"model" binding:"required"`
	Input          interface{} `json:"input" binding:"required"` // Can be string or []string
	EncodingFormat string      `json:"encoding_format,omitempty"`
}

// ImageGenerationRequest represents an image generation request from the user.
type ImageGenerationRequest struct {
	Model          string `json:"model,omitempty"`
	Prompt         string `json:"prompt" binding:"required"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"` // "url" or "b64_json"
}

// ChatCompletion handles chat completion requests.
func (h *ChatHandler) ChatCompletion(c *gin.Context) {
	var req ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		apierror.BadRequest(err.Error()).Respond(c)
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), req.Model)
	if err != nil {
		apierror.ServiceUnavailable("no available providers for model: " + req.Model).Respond(c)
		return
	}

	h.logger.Info("model routed to provider",
		zap.String("model", sanitize.LogValue(req.Model)),
		zap.String("provider", selectedProvider.Name),
		zap.String("base_url", selectedProvider.BaseURL),
	)

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Fetch conversation history if provided
	var historyMessages []provider.Message
	if req.ConversationID != "" && h.memory != nil {
		history, err := h.memory.GetConversationWithLimit(c.Request.Context(), userObj.ID, req.ConversationID, 20)
		if err == nil {
			for _, hm := range history {
				historyMessages = append(historyMessages, provider.Message{Role: hm.Role, Content: provider.StringContent(hm.Content)})
			}
		} else {
			h.logger.Warn("failed to fetch conversation memory", zap.Error(err), zap.String("conversation_id", sanitize.LogValue(req.ConversationID)))
		}
	}

	messages := make([]provider.Message, 0, len(historyMessages)+len(req.Messages))
	messages = append(messages, historyMessages...)
	for _, m := range req.Messages {
		messages = append(messages, provider.Message{Role: m.Role, Content: m.Content})
	}

	// Stream Resume Injection: When upstream crashes, the client can pass a resume pointer containing the last incomplete string.
	// We inject a system directive to guide the model to seamlessly continue.
	if req.ResumeFromStreamID != "" {
		resumeContext := "System Protocol: The previous generation was interrupted due to a network or upstream error. Please continue writing seamlessly from exactly where you left off. Do not repeat anything that was already written. End of System Protocol."
		messages = append(messages, provider.Message{Role: "system", Content: provider.StringContent(resumeContext)})
	}

	providerReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
	}

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "chat_completion", userObj.ID.String(), req.ConversationID, map[string]interface{}{
		"model":  req.Model,
		"stream": req.Stream,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

	// Check user quota before processing
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

	// Handle streaming requests
	if req.Stream {
		// ExecuteStreamChat retries key rotation before the stream is established
		// (before SSE headers are sent), giving streaming the same resilience as non-streaming.
		streamResult, err := h.router.ExecuteStreamChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)
		if err != nil {
			h.logger.Error("failed to establish stream", zap.Error(err))
			c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{
				"message": "upstream provider error",
				"type":    "server_error",
				"code":    "provider_error",
			}})
			return
		}
		h.handleStreamingChat(c, streamResult.Stream, providerReq, selectedProvider, userObj, userAPIKey, start, trace, req.ConversationID, req.Messages)
		return
	}

	// Non-streaming: delegate to Router.ExecuteChat which handles key-rotation retry
	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}, req.Messages)

	result, err := h.router.ExecuteChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:       userObj.ID,
			APIKeyID:     userAPIKey.ID,
			ProviderID:   selectedProvider.ID,
			ModelName:    req.Model,
			Latency:      latency.Milliseconds(),
			StatusCode:   http.StatusBadGateway,
			ErrorMessage: "all API keys failed",
		}
		if err != nil {
			usageLog.ErrorMessage = err.Error()
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		h.logger.Error("provider request failed",
			zap.String("model", sanitize.LogValue(req.Model)),
			zap.String("provider", selectedProvider.Name),
			zap.Error(err),
		)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{
				"message": "upstream provider error",
				"type":    "server_error",
				"code":    "provider_error",
			},
		})
		return
	}

	resp := result.Response
	outText := ""
	if len(resp.Choices) > 0 {
		outText = resp.Choices[0].Message.Content.Text
	}
	gen.End(outText, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	// Save conversation memory
	if req.ConversationID != "" && h.memory != nil {
		for _, m := range req.Messages {
			_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, m.Role, m.Content.Text, 0)
		}
		_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, "assistant", outText, resp.Usage.CompletionTokens)
	}

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

	c.JSON(http.StatusOK, gin.H{
		"id":      resp.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   resp.Model,
		"choices": resp.Choices,
		"usage":   resp.Usage,
	})
}
