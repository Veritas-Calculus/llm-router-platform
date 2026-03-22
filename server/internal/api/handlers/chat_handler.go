// Package handlers provides HTTP request handlers.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/dlp"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/tracking"
	router_errs "llm-router-platform/internal/errors"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/go-redis/redis/v8"
	semantic "llm-router-platform/internal/service/cache"
)

// ChatHandler handles chat completion endpoints.
type ChatHandler struct {
	router     *router.Router
	billing    *billing.Service
	memory     *memory.Service
	subService *billing.SubscriptionService
	balance    *billing.BalanceService
	obsInfo    observability.Service
	db         *gorm.DB
	logger     *zap.Logger
	dispatcher *tracking.Dispatcher
	cache      *semantic.SemanticCacheService
	redis      *redis.Client
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(r *router.Router, b *billing.Service, m *memory.Service, sub *billing.SubscriptionService, bal *billing.BalanceService, obs observability.Service, db *gorm.DB, cacheService *semantic.SemanticCacheService, redisClient *redis.Client, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		router:     r,
		billing:    b,
		memory:     m,
		subService: sub,
		balance:    bal,
		obsInfo:    obs,
		db:         db,
		logger:     logger,
		dispatcher: tracking.NewDispatcher(db, logger),
		cache:      cacheService,
		redis:      redisClient,
	}
}

// checkProjectQuota verifies the project's organization hasn't exceeded their quota.
// Returns nil if within quota, or an error message if exceeded.
func (h *ChatHandler) checkProjectQuota(c *gin.Context, projectObj *models.Project) *string {
	// 1. Check Subscription-based quota
	if h.subService != nil {
		ok, msg, err := h.subService.CheckQuota(c.Request.Context(), projectObj.OrgID)
		if err != nil {
			h.logger.Error("subscription quota check failed", zap.Error(err))
		} else if !ok {
			return &msg
		}
	}

	// Future: Project-specific budget checks can be added here
	return nil
}

// AnthropicMessagesRequest represents an Anthropic messages request.
type AnthropicMessagesRequest struct {
	Model       string                  `json:"model" binding:"required"`
	Messages    []AnthropicMessage      `json:"messages" binding:"required"`
	MaxTokens   int                     `json:"max_tokens" binding:"required"`
	Temperature *float64                `json:"temperature,omitempty"`
	System      string                  `json:"system,omitempty"`
	Stream      bool                    `json:"stream,omitempty"`
	Tools       []AnthropicTool         `json:"tools,omitempty"`
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
	internalMessages := make([]provider.Message, 0)
	
	// Add system message if present
	if anthroReq.System != "" {
		internalMessages = append(internalMessages, provider.Message{
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
			// Simple mapping for complex content blocks
			data, _ := json.Marshal(v)
			content = string(data)
		}
		internalMessages = append(internalMessages, provider.Message{
			Role:    m.Role,
			Content: provider.StringContent(content),
		})
	}

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
	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), anthroReq.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no providers available"})
		return
	}

	projectObj := c.MustGet("project").(*models.Project)
	if quotaErr := h.checkProjectQuota(c, projectObj); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": *quotaErr})
		return
	}

	// For now, only supporting non-streaming for Anthropic compat
	if anthroReq.Stream {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "streaming not yet supported for Anthropic compatible API"})
		return
	}

	start := time.Now()
	result, err := h.router.ExecuteChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider error"})
		return
	}

	resp := result.Response
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
				"text": resp.Choices[0].Message.Content.Text,
			},
		},
		"usage": gin.H{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		},
	}

	// Record usage
	userAPIKey := c.MustGet("api_key").(*models.APIKey)
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
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)
	if h.balance != nil && usageLog.Cost > 0 {
		_ = h.balance.DeductBalance(c.Request.Context(), projectObj.ID, usageLog.Cost, "Anthropic API: "+anthroReq.Model, usageLog.ID.String())
	}

	c.JSON(http.StatusOK, anthroResp)
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
	TrajectoryID       string           `json:"trajectory_id,omitempty"`
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
		c.JSON(http.StatusBadRequest, router_errs.NewRouterError(
			router_errs.ErrCodeProviderParseFailed, http.StatusBadRequest, "invalid_request_error", err.Error(), err,
		).MapToOpenAIResponse())
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusNotFound, router_errs.NewRouterError(
			router_errs.ErrCodeModelNotFound, http.StatusNotFound, "invalid_request_error", "no available providers for model: "+req.Model, err,
		).MapToOpenAIResponse())
		return
	}

	h.logger.Info("model routed to provider",
		zap.String("model", sanitize.LogValue(req.Model)),
		zap.String("provider", selectedProvider.Name),
		zap.String("base_url", selectedProvider.BaseURL),
	)

	projectObj := c.MustGet("project").(*models.Project)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Fetch conversation history if provided
	var historyMessages []provider.Message
	if req.ConversationID != "" && h.memory != nil {
		history, err := h.memory.GetConversationWithLimit(c.Request.Context(), projectObj.ID, req.ConversationID, 20)
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

	// === Data Loss Prevention (DLP) ===
	if projectObj.DlpConfig != nil && projectObj.DlpConfig.IsEnabled {
		for i, m := range messages {
			rawBytes, _ := json.Marshal(m.Content)
			rawStr := string(rawBytes)

			switch projectObj.DlpConfig.Strategy {
			case dlp.StrategyBlock:
				if dlp.HasPII(rawStr, projectObj.DlpConfig) {
					c.JSON(http.StatusBadRequest, router_errs.NewRouterError(
						router_errs.ErrCodeProviderParseFailed, http.StatusBadRequest, "invalid_request_error", "Request blocked by Data Loss Prevention (DLP) policy due to sensitive information.", nil,
					).MapToOpenAIResponse())
					return
				}
			case dlp.StrategyRedact:
				scrubbedStr := dlp.ScrubText(rawStr, projectObj.DlpConfig)
				var newContent provider.FlexibleContent
				_ = json.Unmarshal([]byte(scrubbedStr), &newContent)
				messages[i].Content = newContent
			}
		}
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
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "chat_completion", projectObj.ID.String(), req.ConversationID, map[string]interface{}{
		"model":  req.Model,
		"stream": req.Stream,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

	// Check user quota before processing
	if quotaErr := h.checkProjectQuota(c, projectObj); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, router_errs.NewRouterError(
			router_errs.ErrCodeRateLimitExceeded, http.StatusTooManyRequests, "quota_exceeded", *quotaErr, nil,
		).MapToOpenAIResponse())
		return
	}

	// === Semantic Cache (Exact Match) ===
	var cacheHit *models.SemanticCache
	var promptHash string
	var msgBytes []byte
	if len(messages) > 0 {
		msgBytes, _ = json.Marshal(messages)
		promptHash = h.cache.HashPrompt(string(msgBytes))

		hit, err := h.cache.FindExactMatch(c.Request.Context(), promptHash)
		if err == nil && hit != nil {
			cacheHit = hit
			h.logger.Info("Semantic Cache exact hit", zap.String("hash", promptHash))
		}
	}

	// === Semantic Cache (Semantic Vector Match) ===
	var promptEmbedding []float32
	if cacheHit == nil && promptHash != "" {
		embProvider, embKey, embErr := h.router.Route(c.Request.Context(), "text-embedding-3-small")
		if embErr == nil {
			embReq := &provider.EmbeddingRequest{
				Model: "text-embedding-3-small",
				Input: string(msgBytes),
			}
			embRes, embErr2 := h.router.ExecuteEmbeddings(c.Request.Context(), embProvider, embKey, embReq, 1)
			if embErr2 == nil && len(embRes.Response.Data) > 0 {
				promptEmbedding = embRes.Response.Data[0].Embedding
				
				semanticHit, semErr := h.cache.FindSemanticMatch(c.Request.Context(), promptEmbedding)
				if semErr == nil && semanticHit != nil {
					cacheHit = semanticHit
					h.logger.Info("Semantic Cache vector hit", zap.String("hash", promptHash))
				}
			} else {
                h.logger.Warn("Failed to generate embedding for semantic cache", zap.Error(embErr2))
            }
		} else {
			h.logger.Debug("No embedding provider available for semantic cache (requires text-embedding-3-small)")
		}
	}
	// === End Semantic Match ===

	if cacheHit != nil {
		var cachedResp provider.ChatResponse
		if err := json.Unmarshal(cacheHit.Response, &cachedResp); err == nil {
			
			h.obsInfo.StartGeneration(c.Request.Context(), trace, "Cache: ExactMatch", req.Model, nil, req.Messages).End(cachedResp.Choices[0].Message.Content.Text, 0, 0)
			
			usageLog := &models.UsageLog{
				UserID:         userAPIKey.UserID,
				ProjectID:      projectObj.ID,
		Channel:        userAPIKey.Channel,
				APIKeyID:       userAPIKey.ID,
				ProviderID:     selectedProvider.ID,
				ModelName:      req.Model,
				Latency:        1,
				StatusCode:     http.StatusOK,
				RequestTokens:  0,
				ResponseTokens: 0,
				TotalTokens:    0,
			}
			_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

			if req.Stream {
				c.Writer.Header().Set("Content-Type", "text/event-stream")
				c.Writer.Header().Set("Cache-Control", "no-cache")
				c.Writer.Header().Set("Connection", "keep-alive")

				chunk := provider.StreamChunk{
					ID:    cachedResp.ID,
					Model: cachedResp.Model,
					Choices: []provider.DeltaChoice{
						{
							Delta: provider.Delta{
								Role:    "assistant",
								Content: cachedResp.Choices[0].Message.Content.Text,
							},
							Index:        0,
							FinishReason: "stop",
						},
					},
				}
				chunkBytes, _ := json.Marshal(chunk)
				_, _ = c.Writer.Write([]byte("data: "))
				_, _ = c.Writer.Write(chunkBytes)
				_, _ = c.Writer.Write([]byte("\n\ndata: [DONE]\n\n"))
				c.Writer.Flush()
				return
			}

			c.JSON(http.StatusOK, cachedResp)
			return
		}
	}

	// Handle streaming requests
	if req.Stream {
		// Record initial usage log (pending) to ensure request is tracked even if stream fails early
		usageLog := &models.UsageLog{
			UserID:     userAPIKey.UserID,
			ProjectID:   projectObj.ID,
			APIKeyID:   userAPIKey.ID,
			ProviderID: selectedProvider.ID,
			ModelName:  req.Model,
			Latency:    0,
			StatusCode: http.StatusProcessing, // Temporary status
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		// ExecuteStreamChat retries key rotation before the stream is established
		streamResult, err := h.router.ExecuteStreamChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)
		if err != nil {
			h.saveErrorLog(c.Request.Context(), err, req.TrajectoryID, trace.GetID(), selectedProvider.Name, req.Model)
			h.logger.Error("failed to establish stream", zap.Error(err))
			usageLog.StatusCode = http.StatusBadGateway
			usageLog.ErrorMessage = err.Error()
			_ = h.billing.UpdateUsageTokens(c.Request.Context(), usageLog.ID, 0, 0, http.StatusBadGateway, time.Since(start).Milliseconds(), err.Error())

			c.JSON(http.StatusBadGateway, router_errs.NewRouterError(
				router_errs.ErrCodeInternalSystemError, http.StatusBadGateway, "server_error", "upstream provider error: stream failed to initialize", err,
			).MapToOpenAIResponse())
			return
		}
		h.handleStreamingChat(c, streamResult.Stream, providerReq, selectedProvider, projectObj, userAPIKey, start, trace, req.ConversationID, req.Messages, usageLog.ID, promptHash, promptEmbedding)
		return
	}

	// Non-streaming: delegate to Router.ExecuteChat which handles key-rotation retry
	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
	}, req.Messages)

	result, err := h.router.ExecuteChat(c.Request.Context(), selectedProvider, apiKey, providerReq, 3)

	if err != nil || result == nil {
		if err != nil {
			h.saveErrorLog(c.Request.Context(), err, req.TrajectoryID, trace.GetID(), selectedProvider.Name, req.Model)
		}
		gen.EndWithError(err)
		latency := time.Since(start)
		usageLog := &models.UsageLog{
			UserID:       userAPIKey.UserID,
			ProjectID:   projectObj.ID,
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
		c.JSON(http.StatusBadGateway, router_errs.NewRouterError(
			router_errs.ErrCodeInternalSystemError, http.StatusBadGateway, "server_error", "upstream provider error: request failed", err,
		).MapToOpenAIResponse())
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
		// Save full message history including tool calls and results
		for _, m := range result.FinalMessages {
			content := m.Content.Text
			if content == "" && len(m.ToolCalls) > 0 {
				content = "[Tool Call]"
			}
			_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, req.ConversationID, m.Role, content, 0)
		}
		// Final assistant response
		_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, req.ConversationID, "assistant", outText, resp.Usage.CompletionTokens)
	}

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
		MCPCallCount:   result.MCPCallCount,
		MCPErrorCount:  result.MCPErrorCount,
	}
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	// Deduct balance
	if h.balance != nil && usageLog.Cost > 0 {
		_ = h.balance.DeductBalance(c.Request.Context(), projectObj.ID, usageLog.Cost, "LLM Request: "+req.Model, usageLog.ID.String())
	}

	// Save Semantic Cache (Async)
	if promptHash != "" && len(resp.Choices) > 0 {
		go func(hash string, emb []float32, response interface{}, pid string, m string) {
			if len(emb) == 0 {
				emb = make([]float32, 1536)
			}
			_ = h.cache.StoreCache(context.Background(), hash, emb, response, pid, m, nil)
		}(promptHash, promptEmbedding, resp, selectedProvider.Name, req.Model)
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      resp.ID,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   resp.Model,
		"choices": resp.Choices,
		"usage":   resp.Usage,
	})
}

// saveErrorLog extracts provider.ProviderError and saves an ErrorLog to tracking context.
func (h *ChatHandler) saveErrorLog(ctx context.Context, err error, trajectoryID, traceID, providerName, modelName string) {
	if h.db == nil {
		return
	}

	var provErr *provider.ProviderError
	if errors.As(err, &provErr) {
		headersBytes, _ := json.Marshal(provErr.Headers)
		errLog := &models.ErrorLog{
			ID:           uuid.New(),
			TrajectoryID: trajectoryID,
			TraceID:      traceID,
			Provider:     providerName,
			Model:        modelName,
			StatusCode:   provErr.StatusCode,
			Headers:      headersBytes,
			ResponseBody: provErr.Body,
			CreatedAt:    time.Now(),
		}
		if dbErr := h.db.Create(errLog).Error; dbErr != nil {
			h.logger.Error("failed to save error log", zap.Error(dbErr))
		} else {
			h.dispatcher.ReportRouteError(ctx, errLog)
		}
	}
}
