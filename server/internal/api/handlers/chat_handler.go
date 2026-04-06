// Package handlers provides HTTP request handlers.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/dlp"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/internal/service/safety"
	"llm-router-platform/internal/service/tracking"
	router_errs "llm-router-platform/internal/errors"
	"llm-router-platform/pkg/sanitize"
	"llm-router-platform/pkg/tokencount"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/go-redis/redis/v8"
	semantic "llm-router-platform/internal/service/cache"
)

// ChatHandler handles chat completion endpoints.
type ChatHandler struct {
	router       *router.Router
	billing      *billing.Service
	memory       *memory.Service
	subService   *billing.SubscriptionService
	balance      *billing.BalanceService
	obsInfo      observability.Service
	usageRepo    *repository.UsageLogRepository
	errorLogRepo *repository.ErrorLogRepository
	logger       *zap.Logger
	dispatcher   *tracking.Dispatcher
	cache        *semantic.SemanticCacheService
	redis        *redis.Client
	safety       safety.Classifier
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(r *router.Router, b *billing.Service, m *memory.Service, sub *billing.SubscriptionService, bal *billing.BalanceService, obs observability.Service, db *gorm.DB, cacheService *semantic.SemanticCacheService, redisClient *redis.Client, safetyClassifier safety.Classifier, logger *zap.Logger) *ChatHandler {
	if safetyClassifier == nil {
		safetyClassifier = &safety.NoopClassifier{}
	}
	return &ChatHandler{
		router:       r,
		billing:      b,
		memory:       m,
		subService:   sub,
		balance:      bal,
		obsInfo:      obs,
		usageRepo:    repository.NewUsageLogRepository(db),
		errorLogRepo: repository.NewErrorLogRepository(db),
		logger:       logger,
		dispatcher:   tracking.NewDispatcher(db, logger),
		cache:        cacheService,
		redis:        redisClient,
		safety:       safetyClassifier,
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

	start := time.Now()
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Handle streaming via existing infrastructure
	if anthroReq.Stream {
		h.handleAnthropicStream(c, anthroReq, providerReq, selectedProvider, userAPIKey, projectObj, start)
		return
	}

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
	if err := h.billing.RecordUsageAndDeduct(c.Request.Context(), usageLog, h.balance, projectObj.ID, "Anthropic API: "+anthroReq.Model); err != nil {
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
func (h *ChatHandler) handleAnthropicStream(c *gin.Context, anthroReq AnthropicMessagesRequest, providerReq *provider.ChatRequest, selectedProvider *models.Provider, userAPIKey *models.APIKey, projectObj *models.Project, start time.Time) {
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

	streamResult, err := h.router.ExecuteStreamChat(c.Request.Context(), selectedProvider, nil, providerReq, 3)
	if err != nil {
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
			"id":    "msg_" + uuid.New().String()[:8],
			"type":  "message",
			"role":  "assistant",
			"model": anthroReq.Model,
			"content": []interface{}{},
			"usage": gin.H{"input_tokens": 0, "output_tokens": 0},
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
	for chunk := range streamResult.Stream {
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			totalOutput++
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
	if err := h.billing.UpdateUsageTokens(c.Request.Context(), usageLog.ID, 0, totalOutput, http.StatusOK, latency.Milliseconds(), ""); err != nil {
		h.logger.Warn("billing update failed", zap.Error(err))
	}
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

	// 1. Build messages (conversation history + request)
	messages := h.buildMessages(c, req, projectObj, userAPIKey)

	// 2. Stream resume injection
	if done := h.applyStreamResume(c, &req, projectObj, &messages); done {
		return
	}

	// 3. DLP
	if done := h.applyDLP(c, projectObj, messages); done {
		return
	}

	// 4. Content safety (respects project DLP toggle)
	if done := h.applySafetyCheck(c, req, projectObj, messages); done {
		return
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

	// 5. Quota check
	if quotaErr := h.checkProjectQuota(c, projectObj); quotaErr != nil {
		c.JSON(http.StatusTooManyRequests, router_errs.NewRouterError(
			router_errs.ErrCodeRateLimitExceeded, http.StatusTooManyRequests, "quota_exceeded", *quotaErr, nil,
		).MapToOpenAIResponse())
		return
	}

	// 6. Semantic cache lookup
	msgBytes, _ := json.Marshal(messages)
	promptHash, promptEmbedding, cacheHit := h.lookupSemanticCache(c, messages, msgBytes)

	// 7. Cache hit response
	if cacheHit != nil {
		if h.handleCacheHit(c, cacheHit, req, userAPIKey, selectedProvider, projectObj, msgBytes, trace) {
			return
		}
	}

	// 8. Streaming path
	if req.Stream {
		h.handleStreamPath(c, req, providerReq, selectedProvider, userAPIKey, projectObj, start, trace, promptHash, promptEmbedding)
		return
	}

	// 9. Non-streaming path
	h.handleNonStreamResponse(c, req, providerReq, selectedProvider, apiKey, userAPIKey, projectObj, start, trace, promptHash, promptEmbedding, messages, msgBytes)
}

// ─── ChatCompletion Helpers ────────────────────────────────────────────────

// buildMessages constructs the message list from conversation history + request messages.
func (h *ChatHandler) buildMessages(c *gin.Context, req ChatCompletionRequest, projectObj *models.Project, userAPIKey *models.APIKey) []provider.Message {
	var historyMessages []provider.Message
	if req.ConversationID != "" && h.memory != nil {
		history, err := h.memory.GetConversationWithLimit(c.Request.Context(), projectObj.ID, &userAPIKey.ID, req.ConversationID, 20)
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
	return messages
}

// applyStreamResume handles stream resume injection. Returns true if the request was terminated (bad input).
func (h *ChatHandler) applyStreamResume(c *gin.Context, req *ChatCompletionRequest, projectObj *models.Project, messages *[]provider.Message) bool {
	if req.ResumeFromStreamID == "" {
		return false
	}

	resumeID, parseErr := uuid.Parse(req.ResumeFromStreamID)
	if parseErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "invalid resume_from_stream_id format",
				"type":    "invalid_request_error",
				"code":    "invalid_request",
			},
		})
		return true
	}

	var count int64
	if h.usageRepo != nil {
		count, _ = h.usageRepo.CountInterruptedByIDAndProject(c.Request.Context(), resumeID, projectObj.ID)
	}
	if count > 0 {
		resumeContext := "System Protocol: The previous generation was interrupted due to a network or upstream error. Please continue writing seamlessly from exactly where you left off. Do not repeat anything that was already written. End of System Protocol."
		*messages = append(*messages, provider.Message{Role: "system", Content: provider.StringContent(resumeContext)})
	} else {
		h.logger.Warn("invalid resume_from_stream_id: no matching interrupted stream found",
			zap.String("resume_id", sanitize.LogValue(req.ResumeFromStreamID)),
			zap.String("project_id", projectObj.ID.String()),
		)
	}
	return false
}

// applyDLP runs Data Loss Prevention checks on messages. Returns true if the request was blocked.
func (h *ChatHandler) applyDLP(c *gin.Context, projectObj *models.Project, messages []provider.Message) bool {
	if projectObj.DlpConfig == nil || !projectObj.DlpConfig.IsEnabled {
		return false
	}

	for i, m := range messages {
		rawBytes, _ := json.Marshal(m.Content)
		rawStr := string(rawBytes)

		switch projectObj.DlpConfig.Strategy {
		case dlp.StrategyBlock:
			if dlp.HasPII(rawStr, projectObj.DlpConfig) {
				c.JSON(http.StatusBadRequest, router_errs.NewRouterError(
					router_errs.ErrCodeProviderParseFailed, http.StatusBadRequest, "invalid_request_error", "Request blocked by Data Loss Prevention (DLP) policy due to sensitive information.", nil,
				).MapToOpenAIResponse())
				return true
			}
		case dlp.StrategyRedact:
			scrubbedStr := dlp.ScrubText(rawStr, projectObj.DlpConfig)
			var newContent provider.FlexibleContent
			_ = json.Unmarshal([]byte(scrubbedStr), &newContent)
			messages[i].Content = newContent
		}
	}
	return false
}

// applySafetyCheck runs content safety classification. Returns true if the request was blocked.
// Respects the project-level DLP toggle: when DLP is disabled for a project,
// safety classification is also skipped.
func (h *ChatHandler) applySafetyCheck(c *gin.Context, req ChatCompletionRequest, projectObj *models.Project, messages []provider.Message) bool {
	if h.safety == nil {
		return false
	}

	// Respect project DLP toggle — when DLP is disabled, skip safety classification too
	if projectObj.DlpConfig == nil || !projectObj.DlpConfig.IsEnabled {
		return false
	}

	result, err := h.safety.Classify(c.Request.Context(), messages)
	if err != nil {
		h.logger.Error("safety classification failed", zap.Error(err))
		return false // Fail open
	}

	if !result.Safe {
		h.logger.Warn("request blocked by safety classifier",
			zap.String("category", result.Category),
			zap.Float64("score", result.Score),
			zap.String("model", sanitize.LogValue(req.Model)),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Your request was flagged by our content safety system. Please revise your input.",
				"type":    "invalid_request_error",
				"code":    "content_policy_violation",
			},
		})
		return true
	}
	return false
}

// lookupSemanticCache performs exact-match and vector-match cache lookups. Returns hash, embedding, and cache entry (nil if miss).
func (h *ChatHandler) lookupSemanticCache(c *gin.Context, messages []provider.Message, msgBytes []byte) (string, []float32, *models.SemanticCache) {
	if len(messages) == 0 {
		return "", nil, nil
	}

	promptHash := h.cache.HashPrompt(string(msgBytes))

	// Exact match
	hit, err := h.cache.FindExactMatch(c.Request.Context(), promptHash)
	if err == nil && hit != nil {
		h.logger.Info("Semantic Cache exact hit", zap.String("hash", promptHash))
		return promptHash, nil, hit
	}

	// Vector match
	var promptEmbedding []float32
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
				h.logger.Info("Semantic Cache vector hit", zap.String("hash", promptHash))
				return promptHash, promptEmbedding, semanticHit
			}
		} else {
			h.logger.Warn("Failed to generate embedding for semantic cache", zap.Error(embErr2))
		}
	} else {
		h.logger.Debug("No embedding provider available for semantic cache (requires text-embedding-3-small)")
	}

	return promptHash, promptEmbedding, nil
}

// handleCacheHit serves a cached response (stream or non-stream). Returns true if handled.
func (h *ChatHandler) handleCacheHit(c *gin.Context, cacheHit *models.SemanticCache, req ChatCompletionRequest, userAPIKey *models.APIKey, selectedProvider *models.Provider, projectObj *models.Project, msgBytes []byte, trace observability.Trace) bool {
	var cachedResp provider.ChatResponse
	if err := json.Unmarshal(cacheHit.Response, &cachedResp); err != nil {
		return false
	}

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
		RequestTokens:  tokencount.CountTokens(req.Model, string(msgBytes)),
		ResponseTokens: 0,
		TotalTokens:    tokencount.CountTokens(req.Model, string(msgBytes)),
	}
	if err := h.billing.RecordUsageAndDeduct(c.Request.Context(), usageLog, h.balance, userAPIKey.UserID, fmt.Sprintf("Cache hit: %s", req.Model)); err != nil {
		h.logger.Warn("billing deduction failed (cache hit)", zap.Error(err), zap.String("model", sanitize.LogValue(req.Model)))
	}

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
		return true
	}

	c.JSON(http.StatusOK, cachedResp)
	return true
}

// handleStreamPath handles the streaming chat path (pre-record, establish stream, delegate).
func (h *ChatHandler) handleStreamPath(c *gin.Context, req ChatCompletionRequest, providerReq *provider.ChatRequest, selectedProvider *models.Provider, userAPIKey *models.APIKey, projectObj *models.Project, start time.Time, trace observability.Trace, promptHash string, promptEmbedding []float32) {
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:  projectObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  req.Model,
		Latency:    0,
		StatusCode: http.StatusProcessing,
	}
	if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
		h.logger.Warn("billing pre-record failed", zap.Error(err), zap.String("model", sanitize.LogValue(req.Model)))
	}

	streamResult, err := h.router.ExecuteStreamChat(c.Request.Context(), selectedProvider, nil, providerReq, 3)
	if err != nil {
		h.saveErrorLog(c.Request.Context(), err, req.TrajectoryID, trace.GetID(), selectedProvider.Name, req.Model)
		h.logger.Error("failed to establish stream", zap.Error(err))
		usageLog.StatusCode = http.StatusBadGateway
		usageLog.ErrorMessage = sanitize.TruncateErrorMessage(err.Error())
		if billingErr := h.billing.UpdateUsageTokens(c.Request.Context(), usageLog.ID, 0, 0, http.StatusBadGateway, time.Since(start).Milliseconds(), sanitize.TruncateErrorMessage(err.Error())); billingErr != nil {
			h.logger.Warn("billing update failed", zap.Error(billingErr))
		}

		c.JSON(http.StatusBadGateway, router_errs.NewRouterError(
			router_errs.ErrCodeInternalSystemError, http.StatusBadGateway, "server_error", "upstream provider error: stream failed to initialize", err,
		).MapToOpenAIResponse())
		return
	}
	h.handleStreamingChat(c, streamResult.Stream, providerReq, selectedProvider, projectObj, userAPIKey, start, trace, req.ConversationID, req.Messages, usageLog.ID, promptHash, promptEmbedding)
}

// handleNonStreamResponse handles non-streaming chat completion, billing, memory save, and cache store.
func (h *ChatHandler) handleNonStreamResponse(c *gin.Context, req ChatCompletionRequest, providerReq *provider.ChatRequest, selectedProvider *models.Provider, apiKey *models.ProviderAPIKey, userAPIKey *models.APIKey, projectObj *models.Project, start time.Time, trace observability.Trace, promptHash string, promptEmbedding []float32, messages []provider.Message, msgBytes []byte) {
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
			ProjectID:    projectObj.ID,
			APIKeyID:     userAPIKey.ID,
			ProviderID:   selectedProvider.ID,
			ModelName:    req.Model,
			Latency:      latency.Milliseconds(),
			StatusCode:   http.StatusBadGateway,
			ErrorMessage: "all API keys failed",
		}
		if err != nil {
			usageLog.ErrorMessage = sanitize.TruncateErrorMessage(err.Error())
		}
		if err := h.billing.RecordUsage(c.Request.Context(), usageLog); err != nil {
			h.logger.Warn("billing pre-record failed", zap.Error(err), zap.String("model", sanitize.LogValue(req.Model)))
		}

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
		for _, m := range result.FinalMessages {
			content := m.Content.Text
			if content == "" && len(m.ToolCalls) > 0 {
				content = "[Tool Call]"
			}
			_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, &userAPIKey.ID, req.ConversationID, m.Role, content, 0)
		}
		_ = h.memory.AddMessage(c.Request.Context(), projectObj.ID, &userAPIKey.ID, req.ConversationID, "assistant", outText, resp.Usage.CompletionTokens)
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
	if err := h.billing.RecordUsageAndDeduct(c.Request.Context(), usageLog, h.balance, projectObj.ID, "LLM Request: "+req.Model); err != nil {
		h.logger.Warn("billing deduction failed", zap.Error(err), zap.String("model", sanitize.LogValue(req.Model)))
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

// saveErrorLog extracts provider.ProviderError and saves an ErrorLog via the repository.
func (h *ChatHandler) saveErrorLog(ctx context.Context, err error, trajectoryID, traceID, providerName, modelName string) {
	if h.errorLogRepo == nil {
		return
	}

	var provErr *provider.ProviderError
	if errors.As(err, &provErr) {
		sanitizedHeaders := sanitize.RedactHeaders(provErr.Headers)
		sanitizedBody := sanitize.TruncateResponseBody(provErr.Body)
		errLog := &models.ErrorLog{
			ID:           uuid.New(),
			TrajectoryID: trajectoryID,
			TraceID:      traceID,
			Provider:     providerName,
			Model:        modelName,
			StatusCode:   provErr.StatusCode,
			Headers:      sanitizedHeaders,
			ResponseBody: sanitizedBody,
			CreatedAt:    time.Now(),
		}
		if dbErr := h.errorLogRepo.Create(ctx, errLog); dbErr != nil {
			h.logger.Error("failed to save error log", zap.Error(dbErr))
		} else {
			h.dispatcher.ReportRouteError(ctx, errLog)
		}
	}
}

// handleProviderError records a usage log for a failed provider request and sends the
// appropriate error response. This shared helper deduplicates logic between Embeddings
// and TranscribeAudio handlers.
func (h *ChatHandler) handleProviderError(c *gin.Context, err error, start time.Time, userAPIKey *models.APIKey, projectObj *models.Project, selectedProvider *models.Provider, modelName string) {
	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:     userAPIKey.UserID,
		ProjectID:  projectObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  modelName,
		Latency:    latency.Milliseconds(),
		StatusCode: http.StatusBadGateway,
	}
	if err != nil {
		usageLog.ErrorMessage = sanitize.TruncateErrorMessage(err.Error())
		if err == provider.ErrNotImplemented {
			usageLog.StatusCode = http.StatusNotImplemented
		}
	} else {
		usageLog.ErrorMessage = "all API keys failed"
	}
	if billingErr := h.billing.RecordUsage(c.Request.Context(), usageLog); billingErr != nil {
		h.logger.Warn("billing record failed", zap.Error(billingErr))
	}

	if err == provider.ErrNotImplemented {
		c.JSON(http.StatusNotImplemented, gin.H{"error": modelName + " not supported by this provider"})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
}
