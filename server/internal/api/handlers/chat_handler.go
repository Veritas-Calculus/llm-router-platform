// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// isQuotaOrRateLimitError checks if an error message indicates a quota or rate limit issue.
func isQuotaOrRateLimitError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	quotaKeywords := []string{
		"quota",
		"rate limit",
		"rate_limit",
		"ratelimit",
		"too many requests",
		"429",
		"insufficient_quota",
		"billing",
		"exceeded",
		"limit reached",
		"resource exhausted",
		"resourceexhausted",
	}
	for _, keyword := range quotaKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}

// ChatHandler handles chat completion endpoints.
type ChatHandler struct {
	router  *router.Router
	billing *billing.Service
	memory  *memory.Service
	logger  *zap.Logger
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(r *router.Router, b *billing.Service, m *memory.Service, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		router:  r,
		billing: b,
		memory:  m,
		logger:  logger,
	}
}

// ChatCompletionRequest represents a chat completion request.
type ChatCompletionRequest struct {
	Model          string           `json:"model" binding:"required"`
	Messages       []MessageRequest `json:"messages" binding:"required"`
	MaxTokens      int              `json:"max_tokens,omitempty"`
	Temperature    float64          `json:"temperature,omitempty"`
	Stream         bool             `json:"stream,omitempty"`
	ConversationID string           `json:"conversation_id,omitempty"`
}

// MessageRequest represents a message in the request.
type MessageRequest struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ChatCompletion handles chat completion requests.
func (h *ChatHandler) ChatCompletion(c *gin.Context) {
	var req ChatCompletionRequest
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

	messages := make([]provider.Message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = provider.Message{Role: m.Role, Content: m.Content}
	}

	providerReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Handle streaming requests
	if req.Stream {
		// For streaming, try with the first key; if it fails, the stream handler will manage
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, apiKey)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}
		h.handleStreamingChat(c, client, providerReq, selectedProvider, userObj, userAPIKey, start)
		return
	}

	// Non-streaming: try with API key pooling (retry with different keys on failure)
	maxRetries := 3
	var resp *provider.ChatResponse
	var lastErr error
	currentAPIKey := apiKey

	for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
			lastErr = err
			// Try next key
			currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
			continue
		}

		resp, err = client.Chat(c.Request.Context(), providerReq)
		if err != nil {
			lastErr = err
			h.logger.Warn("chat request failed, trying next API key",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.String("key_prefix", currentAPIKey.KeyPrefix),
			)

			// Check if this is a rate limit or quota error
			errStr := err.Error()
			if isQuotaOrRateLimitError(errStr) {
				// Mark this key as temporarily failed
				h.router.MarkKeyFailed(currentAPIKey.ID, errStr)
			}

			// Try next key
			currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
			continue
		}

		// Success - clear any previous failure for this key
		h.router.ClearKeyFailure(currentAPIKey.ID)
		break
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
			usageLog.ErrorMessage = lastErr.Error()
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	usageLog.RequestTokens = resp.Usage.PromptTokens
	usageLog.ResponseTokens = resp.Usage.CompletionTokens
	usageLog.TotalTokens = resp.Usage.TotalTokens
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	_ = apiKey

	c.JSON(http.StatusOK, gin.H{
		"id":      resp.ID,
		"model":   resp.Model,
		"choices": resp.Choices,
		"usage":   resp.Usage,
	})
}

// handleStreamingChat handles streaming chat completion requests.
func (h *ChatHandler) handleStreamingChat(c *gin.Context, client provider.Client, req *provider.ChatRequest, selectedProvider *models.Provider, userObj *models.User, userAPIKey *models.APIKey, start time.Time) {
	chunks, err := client.StreamChat(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed"})
		return
	}

	// Set headers for SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	c.Stream(func(w io.Writer) bool {
		chunk, ok := <-chunks
		if !ok {
			return false
		}

		if chunk.Error != nil {
			return false
		}

		if chunk.Done {
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return false
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
	latency := time.Since(start)
	usageLog := &models.UsageLog{
		UserID:     userObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  req.Model,
		Latency:    latency.Milliseconds(),
		StatusCode: http.StatusOK,
	}
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)
}

// ModelHandler handles model listing endpoints.
type ModelHandler struct {
	registry *provider.Registry
	logger   *zap.Logger
}

// NewModelHandler creates a new model handler.
func NewModelHandler(registry *provider.Registry, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{
		registry: registry,
		logger:   logger,
	}
}

// List returns available models.
func (h *ModelHandler) List(c *gin.Context) {
	providerNames := h.registry.List()
	allModels := make([]provider.ModelInfo, 0)

	for _, name := range providerNames {
		client, ok := h.registry.Get(name)
		if !ok {
			continue
		}

		models, err := client.ListModels(c.Request.Context())
		if err != nil {
			h.logger.Error("failed to list models", zap.String("provider", name), zap.Error(err))
			continue
		}

		allModels = append(allModels, models...)
	}

	c.JSON(http.StatusOK, gin.H{"models": allModels})
}

// UsageHandler handles usage statistics endpoints.
type UsageHandler struct {
	billing *billing.Service
	logger  *zap.Logger
}

// NewUsageHandler creates a new usage handler.
func NewUsageHandler(billing *billing.Service, logger *zap.Logger) *UsageHandler {
	return &UsageHandler{
		billing: billing,
		logger:  logger,
	}
}

// GetSummary returns usage summary.
func (h *UsageHandler) GetSummary(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, -1, 0)

	summary, err := h.billing.GetUsageSummary(c.Request.Context(), id, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetDaily returns daily usage statistics.
func (h *UsageHandler) GetDaily(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	daily, err := h.billing.GetDailyUsage(c.Request.Context(), id, 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": daily})
}

// GetByProvider returns usage by provider.
func (h *UsageHandler) GetByProvider(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, -1, 0)

	usage, err := h.billing.GetUsageByProvider(c.Request.Context(), id, startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, usage)
}

// GetRecent returns recent usage logs.
func (h *UsageHandler) GetRecent(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(userID.(string))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	logs, err := h.billing.GetRecentUsage(c.Request.Context(), id, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs, "total": len(logs)})
}
