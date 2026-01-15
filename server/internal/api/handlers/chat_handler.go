// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
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

	client, ok := h.router.GetProviderClient(selectedProvider.Name)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client not found"})
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

	resp, err := client.Chat(c.Request.Context(), providerReq)
	latency := time.Since(start)

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	usageLog := &models.UsageLog{
		UserID:     userObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		Latency:    latency.Milliseconds(),
	}

	if err != nil {
		usageLog.StatusCode = http.StatusBadGateway
		usageLog.ErrorMessage = err.Error()
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed"})
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
