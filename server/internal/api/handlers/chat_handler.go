// Package handlers provides HTTP request handlers.
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/internal/service/memory"
	"llm-router-platform/internal/service/observability"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"
	"llm-router-platform/pkg/sanitize"

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
		return nil // fail open — don't block on error
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
	Messages           []MessageRequest `json:"messages" binding:"required"`
	MaxTokens          int              `json:"max_tokens,omitempty"`
	Temperature        float64          `json:"temperature,omitempty"`
	Stream             bool             `json:"stream,omitempty"`
	ConversationID     string           `json:"conversation_id,omitempty"`
	ResumeFromStreamID string           `json:"resume_from_stream_id,omitempty"` // For resuming broken streams
}

// MessageRequest represents a message in the request.
type MessageRequest struct {
	Role    string `json:"role" binding:"required"`
	Content string `json:"content" binding:"required"`
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Fetch conversation history if provided
	var historyMessages []provider.Message
	if req.ConversationID != "" && h.memory != nil {
		history, err := h.memory.GetConversationWithLimit(c.Request.Context(), userObj.ID, req.ConversationID, 20)
		if err == nil {
			for _, hm := range history {
				historyMessages = append(historyMessages, provider.Message{Role: hm.Role, Content: hm.Content})
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
		messages = append(messages, provider.Message{Role: "system", Content: resumeContext})
	}

	providerReq := &provider.ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
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
		// For streaming, try with the first key; if it fails, the stream handler will manage
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, apiKey)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}
		h.handleStreamingChat(c, client, providerReq, selectedProvider, userObj, userAPIKey, start, trace, req.ConversationID, req.Messages)
		return
	}

	// Non-streaming: try with API key pooling (retry with different keys on failure)
	maxRetries := 3
	var resp *provider.ChatResponse
	var lastErr error
	currentAPIKey := apiKey

	// For providers that don't require API keys, we still need to make a request
	if !selectedProvider.RequiresAPIKey {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, nil)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}

		genName := "Provider: " + selectedProvider.Name
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
			"temperature": req.Temperature,
			"max_tokens":  req.MaxTokens,
		}, req.Messages)

		resp, lastErr = client.Chat(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			// calculate tokens if we can
			outText := ""
			if len(resp.Choices) > 0 {
				outText = resp.Choices[0].Message.Content
			}
			gen.End(outText, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

			if req.ConversationID != "" && h.memory != nil {
				for _, m := range req.Messages {
					_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, m.Role, m.Content, 0)
				}
				_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, "assistant", outText, resp.Usage.CompletionTokens)
			}
		}
	} else {
		// For providers that require API keys, retry with different keys on failure
		for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
			client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
			if err != nil {
				h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
				lastErr = err
				// Try next key
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			genName := "Provider: " + selectedProvider.Name
			if attempt > 0 {
				genName += fmt.Sprintf(" (Retry %d)", attempt)
			}
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
				"temperature": req.Temperature,
				"max_tokens":  req.MaxTokens,
			}, req.Messages)

			resp, err = client.Chat(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
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
			outText := ""
			if resp != nil && len(resp.Choices) > 0 {
				outText = resp.Choices[0].Message.Content
			}
			if resp != nil {
				gen.End(outText, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				if req.ConversationID != "" && h.memory != nil {
					for _, m := range req.Messages {
						_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, m.Role, m.Content, 0)
					}
					_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, req.ConversationID, "assistant", outText, resp.Usage.CompletionTokens)
				}
			}
			h.router.ClearKeyFailure(currentAPIKey.ID)
			break
		}
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
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   resp.Model,
		"choices": resp.Choices,
		"usage":   resp.Usage,
	})
}

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

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "embeddings", userObj.ID.String(), "", map[string]interface{}{
		"model":           req.Model,
		"encoding_format": req.EncodingFormat,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

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

	maxRetries := 3
	var resp *provider.EmbeddingResponse
	var lastErr error
	currentAPIKey := apiKey

	if !selectedProvider.RequiresAPIKey {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, nil)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}

		genName := "Provider: " + selectedProvider.Name
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
			"encoding_format": req.EncodingFormat,
		}, req.Input)

		resp, lastErr = client.Embeddings(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
		}
	} else {
		for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
			client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
			if err != nil {
				h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
				lastErr = err
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			genName := "Provider: " + selectedProvider.Name
			if attempt > 0 {
				genName += fmt.Sprintf(" (Retry %d)", attempt)
			}
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, req.Model, map[string]interface{}{
				"encoding_format": req.EncodingFormat,
			}, req.Input)

			resp, err = client.Embeddings(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
				lastErr = err
				h.logger.Warn("embeddings request failed, trying next API key",
					zap.Error(err),
					zap.Int("attempt", attempt+1),
					zap.String("key_prefix", currentAPIKey.KeyPrefix),
				)

				errStr := err.Error()
				if isQuotaOrRateLimitError(errStr) {
					h.router.MarkKeyFailed(currentAPIKey.ID, errStr)
				}
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			if resp != nil {
				gen.End("Embedded representation generated successfully", resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			}
			h.router.ClearKeyFailure(currentAPIKey.ID)
			break
		}
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
			if lastErr == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
			usageLog.ErrorMessage = lastErr.Error()
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		if lastErr == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "embeddings not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	usageLog.RequestTokens = resp.Usage.PromptTokens
	usageLog.ResponseTokens = resp.Usage.CompletionTokens
	usageLog.TotalTokens = resp.Usage.TotalTokens
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	c.JSON(http.StatusOK, resp)
}

// GenerateImage handles image generation requests.
func (h *ChatHandler) GenerateImage(c *gin.Context) {
	var req ImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Model might be missing if prompt provided directly, default to dall-e-3
	model := req.Model
	if model == "" {
		model = "dall-e-3"
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	providerReq := &provider.ImageGenerationRequest{
		Model:          model,
		Prompt:         req.Prompt,
		N:              req.N,
		Size:           req.Size,
		ResponseFormat: req.ResponseFormat,
	}

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "generate_image", userObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"size":            req.Size,
		"response_format": req.ResponseFormat,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

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

	maxRetries := 3
	var resp *provider.ImageGenerationResponse
	var lastErr error
	currentAPIKey := apiKey

	if !selectedProvider.RequiresAPIKey {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, nil)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}
		genName := "Provider: " + selectedProvider.Name
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
			"size":            req.Size,
			"response_format": req.ResponseFormat,
			"n":               req.N,
		}, req.Prompt)

		resp, lastErr = client.GenerateImage(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			gen.End("Image generated successfully", 0, 0)
		}
	} else {
		for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
			client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
			if err != nil {
				h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
				lastErr = err
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			genName := "Provider: " + selectedProvider.Name
			if attempt > 0 {
				genName += fmt.Sprintf(" (Retry %d)", attempt)
			}
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
				"size":            req.Size,
				"response_format": req.ResponseFormat,
				"n":               req.N,
			}, req.Prompt)

			resp, err = client.GenerateImage(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
				lastErr = err
				h.logger.Warn("image generation request failed, trying next API key",
					zap.Error(err),
					zap.Int("attempt", attempt+1),
					zap.String("key_prefix", currentAPIKey.KeyPrefix),
				)

				errStr := err.Error()
				if isQuotaOrRateLimitError(errStr) {
					h.router.MarkKeyFailed(currentAPIKey.ID, errStr)
				}
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}
			if resp != nil {
				gen.End("Image generated successfully", 0, 0)
			}
			h.router.ClearKeyFailure(currentAPIKey.ID)
			break
		}
	}

	latency := time.Since(start)

	usageLog := &models.UsageLog{
		UserID:     userObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  model,
		Latency:    latency.Milliseconds(),
	}

	if resp == nil {
		usageLog.StatusCode = http.StatusBadGateway
		if lastErr != nil {
			if lastErr == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
			usageLog.ErrorMessage = lastErr.Error()
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		if lastErr == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "image generation not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	// Image requests are often billed differently, but we log the request.
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	c.JSON(http.StatusOK, resp)
}

// TranscribeAudio handles audio transcription requests.
func (h *ChatHandler) TranscribeAudio(c *gin.Context) {
	file, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required: " + err.Error()})
		return
	}
	defer func() { _ = file.Close() }()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	model := c.PostForm("model")
	if model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
		return
	}

	start := time.Now()

	selectedProvider, apiKey, err := h.router.Route(c.Request.Context(), model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available providers"})
		return
	}

	// Read optional fields
	var temperature float64
	tempStr := c.PostForm("temperature")
	if tempStr != "" {
		_, _ = fmt.Sscanf(tempStr, "%f", &temperature)
	}

	providerReq := &provider.AudioTranscriptionRequest{
		File:           fileBytes,
		FileName:       fileHeader.Filename,
		Model:          model,
		Language:       c.PostForm("language"),
		Prompt:         c.PostForm("prompt"),
		ResponseFormat: c.PostForm("response_format"),
		Temperature:    temperature,
	}

	userObj := c.MustGet("user").(*models.User)
	userAPIKey := c.MustGet("api_key").(*models.APIKey)

	// Observability: Start Trace
	reqID := c.GetHeader("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}
	trace := h.obsInfo.StartTrace(c.Request.Context(), reqID, "transcribe_audio", userObj.ID.String(), "", map[string]interface{}{
		"model":           model,
		"language":        providerReq.Language,
		"response_format": providerReq.ResponseFormat,
		"temperature":     providerReq.Temperature,
		"filename":        providerReq.FileName,
	})
	c.Header("X-Langfuse-Trace-Id", trace.GetID())
	defer trace.End()

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

	maxRetries := 3
	var resp *provider.AudioTranscriptionResponse
	var lastErr error
	currentAPIKey := apiKey

	if !selectedProvider.RequiresAPIKey {
		client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, nil)
		if err != nil {
			h.logger.Error("failed to create provider client", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provider client creation failed"})
			return
		}
		genName := "Provider: " + selectedProvider.Name
		gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
			"language":        providerReq.Language,
			"response_format": providerReq.ResponseFormat,
			"temperature":     providerReq.Temperature,
		}, providerReq.Prompt)

		resp, lastErr = client.TranscribeAudio(c.Request.Context(), providerReq)
		if lastErr != nil {
			gen.EndWithError(lastErr)
		} else if resp != nil {
			gen.End(resp.Text, 0, 0)
		}
	} else {
		for attempt := 0; attempt < maxRetries && currentAPIKey != nil; attempt++ {
			client, err := h.router.GetProviderClientWithKey(c.Request.Context(), selectedProvider, currentAPIKey)
			if err != nil {
				h.logger.Error("failed to create provider client", zap.Error(err), zap.Int("attempt", attempt+1))
				lastErr = err
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}

			genName := "Provider: " + selectedProvider.Name
			if attempt > 0 {
				genName += fmt.Sprintf(" (Retry %d)", attempt)
			}
			gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, genName, model, map[string]interface{}{
				"language":        providerReq.Language,
				"response_format": providerReq.ResponseFormat,
				"temperature":     providerReq.Temperature,
			}, providerReq.Prompt)

			resp, err = client.TranscribeAudio(c.Request.Context(), providerReq)
			if err != nil {
				gen.EndWithError(err)
				lastErr = err
				h.logger.Warn("audio transcription request failed, trying next API key",
					zap.Error(err),
					zap.Int("attempt", attempt+1),
					zap.String("key_prefix", currentAPIKey.KeyPrefix),
				)

				errStr := err.Error()
				if isQuotaOrRateLimitError(errStr) {
					h.router.MarkKeyFailed(currentAPIKey.ID, errStr)
				}
				currentAPIKey, _ = h.router.SelectNextAPIKey(c.Request.Context(), selectedProvider.ID, currentAPIKey.ID)
				continue
			}
			if resp != nil {
				gen.End(resp.Text, 0, 0)
			}
			h.router.ClearKeyFailure(currentAPIKey.ID)
			break
		}
	}

	latency := time.Since(start)

	usageLog := &models.UsageLog{
		UserID:     userObj.ID,
		APIKeyID:   userAPIKey.ID,
		ProviderID: selectedProvider.ID,
		ModelName:  model,
		Latency:    latency.Milliseconds(),
	}

	if resp == nil {
		usageLog.StatusCode = http.StatusBadGateway
		if lastErr != nil {
			if lastErr == provider.ErrNotImplemented {
				usageLog.StatusCode = http.StatusNotImplemented
			}
			usageLog.ErrorMessage = lastErr.Error()
		} else {
			usageLog.ErrorMessage = "all API keys failed"
		}
		_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

		if lastErr == provider.ErrNotImplemented {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "audio transcription not supported by this provider"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed after retries"})
		return
	}

	usageLog.StatusCode = http.StatusOK
	_ = h.billing.RecordUsage(c.Request.Context(), usageLog)

	// In OpenAI's API, the text format requests return plain text string directly.
	// The client provider wrapper handles format translation into the unified struct.
	if providerReq.ResponseFormat == "text" || providerReq.ResponseFormat == "srt" || providerReq.ResponseFormat == "vtt" {
		c.String(http.StatusOK, resp.Text)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// handleStreamingChat handles streaming chat completion requests.
func (h *ChatHandler) handleStreamingChat(c *gin.Context, client provider.Client, req *provider.ChatRequest, selectedProvider *models.Provider, userObj *models.User, userAPIKey *models.APIKey, start time.Time, trace observability.Trace, conversationID string, originalMessages []MessageRequest) {
	gen := h.obsInfo.StartGeneration(c.Request.Context(), trace, "Provider: "+selectedProvider.Name, req.Model, map[string]interface{}{
		"temperature": req.Temperature,
		"max_tokens":  req.MaxTokens,
		"stream":      true,
	}, req.Messages)

	chunks, err := client.StreamChat(c.Request.Context(), req)
	if err != nil {
		gen.EndWithError(err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "provider request failed"})
		return
	}

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
	// By default, if the stream hasn't produced token usage, they are passed as 0 and Langfuse tokenizer calculates them.
	// If the provider supports `include_usage`, we now accurately bill based on these values.
	gen.End(fullText, promptTokens, completionTokens)

	if conversationID != "" && h.memory != nil {
		for _, m := range originalMessages {
			_ = h.memory.AddMessage(c.Request.Context(), userObj.ID, conversationID, m.Role, m.Content, 0)
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
