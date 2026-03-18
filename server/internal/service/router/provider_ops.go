// Package router provides LLM request routing logic.
// This file contains provider CRUD operations, client creation, and health checks.
package router

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ChatResult contains the result of an ExecuteChat call.
type ChatResult struct {
	Response      *provider.ChatResponse
	UsedKey       *models.ProviderAPIKey // nil for providers that don't require keys
	FinalMessages []provider.Message     // Final list of messages after tool call loops
	MCPCallCount  int
	MCPErrorCount int
}

// ExecuteChat sends a chat request to the given provider with automatic key-rotation retry.
// For providers that don't require API keys, it makes a single attempt.
// For providers that require API keys, it retries with different keys on failure (up to maxRetries).
// This centralizes the retry/key-failure logic that was previously in the HTTP handler.
func (r *Router) ExecuteChat(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.ChatRequest, maxRetries int) (*ChatResult, error) {
	if !r.IsProviderHealthy(p.ID) {
		return nil, errors.New("provider is temporarily unavailable (circuit-breaker)")
	}

	// Phase 2: Inject MCP Tools
	r.injectMCPTools(ctx, req)

	if !p.RequiresAPIKey {
		res, err := r.executeChatWithMCP(ctx, p, nil, req)
		if err != nil && isProviderLevelError(err.Error()) {
			r.MarkProviderFailure(p.ID)
		} else if err == nil {
			r.MarkProviderSuccess(p.ID)
		}
		return res, err
	}

	currentKey := apiKey
	var lastErr error

	for attempt := 0; attempt < maxRetries && currentKey != nil; attempt++ {
		result, err := r.executeChatWithMCP(ctx, p, currentKey, req)
		if err == nil {
			r.ClearKeyFailure(currentKey.ID)
			r.MarkProviderSuccess(p.ID)
			return result, nil
		}

		lastErr = err
		r.logger.Warn("chat request failed, trying next API key",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.String("provider", p.Name),
		)

		// Mark key as failed if it's a quota/rate-limit error
		if isQuotaOrRateLimitError(err.Error()) {
			r.MarkKeyFailed(currentKey.ID, err.Error())
		} else if isProviderLevelError(err.Error()) {
			r.MarkProviderFailure(p.ID)
		}

		// Try next key
		currentKey, _ = r.SelectNextAPIKey(ctx, p.ID, currentKey.ID)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("all API keys failed")
}

// executeChatWithMCP wraps executeChatOnce with MCP tool handling feedback loop.
func (r *Router) executeChatWithMCP(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.ChatRequest) (*ChatResult, error) {
	messages := make([]provider.Message, len(req.Messages))
	copy(messages, req.Messages)

	var totalMCPCalls int
	var totalMCPErrors int

	// Max 5 loops for tool calls to prevent infinite loops
	for loop := 0; loop < 5; loop++ {
		result, err := r.executeChatOnce(ctx, p, apiKey, req)
		if err != nil {
			return nil, err
		}

		// Update current messages in the request for next potential loop
		anyMCPHandled, mcpCalls, mcpErrors, err := r.handleMCPToolCalls(ctx, result.Response, &messages)
		if err != nil {
			return nil, err
		}

		totalMCPCalls += mcpCalls
		totalMCPErrors += mcpErrors

		if !anyMCPHandled {
			result.FinalMessages = messages
			result.MCPCallCount = totalMCPCalls
			result.MCPErrorCount = totalMCPErrors
			return result, nil
		}

		// Update request messages and repeat
		req.Messages = messages
		r.logger.Info("repeating LLM request after MCP tool execution", 
			zap.String("provider", p.Name), 
			zap.Int("loop", loop+1))
	}

	return nil, errors.New("too many MCP tool call loops")
}

// ─── Support Functions ─────────────────────────────────────────────────────

// injectMCPTools fetches active MCP tools and adds them to the request if none are present.
func (r *Router) injectMCPTools(ctx context.Context, req *provider.ChatRequest) {
	if r.mcpService == nil {
		return
	}

	// Only inject if no tools are currently specified in the request
	if len(req.Tools) > 0 {
		return
	}

	tools, err := r.mcpService.GetToolsForLLM(ctx)
	if err != nil || len(tools) == 0 {
		return
	}

	toolsJSON, err := json.Marshal(tools)
	if err != nil {
		return
	}

	req.Tools = toolsJSON
}

// handleMCPToolCalls intercept and executes MCP tool calls, returning true if any were handled.
func (r *Router) handleMCPToolCalls(ctx context.Context, resp *provider.ChatResponse, messages *[]provider.Message) (bool, int, int, error) {
	if r.mcpService == nil || len(resp.Choices) == 0 {
		return false, 0, 0, nil
	}

	choice := resp.Choices[0]
	if len(choice.Message.ToolCalls) == 0 {
		return false, 0, 0, nil
	}

	var toolCalls []struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"function"`
	}

	if err := json.Unmarshal(choice.Message.ToolCalls, &toolCalls); err != nil {
		return false, 0, 0, err
	}

	// Add assistant message with tool calls to history
	*messages = append(*messages, choice.Message)

	anyMCPHandled := false
	mcpCalls := 0
	mcpErrors := 0
	for _, tc := range toolCalls {
		if !strings.Contains(tc.Function.Name, "__") {
			// Not an MCP tool (might be user-defined or other)
			continue
		}

		parts := strings.SplitN(tc.Function.Name, "__", 2)
		if len(parts) != 2 {
			continue
		}

		serverName, toolName := parts[0], parts[1]
		
		var args interface{}
		_ = json.Unmarshal(tc.Function.Arguments, &args)

		r.logger.Info("executing MCP tool", zap.String("server", serverName), zap.String("tool", toolName))
		mcpCalls++
		result, err := r.mcpService.CallTool(ctx, serverName, toolName, args)
		
		resultJSON, _ := json.Marshal(result)
		if err != nil {
			mcpErrors++
			resultJSON, _ = json.Marshal(map[string]string{"error": err.Error()})
		}

		// Add tool result message
		*messages = append(*messages, provider.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Function.Name,
			Content:    provider.StringContent(string(resultJSON)),
		})
		anyMCPHandled = true
	}

	return anyMCPHandled, mcpCalls, mcpErrors, nil
}

// isProviderLevelError checks if an error should trigger provider circuit breaking (e.g. 500, timeout).
func isProviderLevelError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	providerKeywords := []string{
		"timeout", "deadline exceeded", "connection refused",
		"500", "502", "503", "504", "internal server error",
		"bad gateway", "service unavailable", "gateway timeout",
	}
	for _, keyword := range providerKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}

// executeChatOnce makes a single chat request using the given provider and key.
func (r *Router) executeChatOnce(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.ChatRequest) (*ChatResult, error) {
	client, err := r.GetProviderClientWithKey(ctx, p, apiKey)
	if err != nil {
		return nil, err
	}

	resp, err := client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}

	return &ChatResult{Response: resp, UsedKey: apiKey}, nil
}

// isQuotaOrRateLimitError checks if an error message indicates a quota or rate limit issue.
func isQuotaOrRateLimitError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	quotaKeywords := []string{
		"quota", "rate limit", "rate_limit", "ratelimit",
		"too many requests", "429", "insufficient_quota",
		"billing", "exceeded", "limit reached",
		"resource exhausted", "resourceexhausted",
	}
	for _, keyword := range quotaKeywords {
		if strings.Contains(errLower, keyword) {
			return true
		}
	}
	return false
}

// executeWithKeyRetry runs fn with automatic key-rotation retry.
// fn receives a provider.Client and should make a single request.
// If the provider doesn't require API keys, fn is called once with a keyless client.
func (r *Router) executeWithKeyRetry(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, maxRetries int, fn func(client provider.Client) error) (*models.ProviderAPIKey, error) {
	if !p.RequiresAPIKey {
		client, err := r.GetProviderClientWithKey(ctx, p, nil)
		if err != nil {
			return nil, err
		}
		return nil, fn(client)
	}

	currentKey := apiKey
	var lastErr error

	for attempt := 0; attempt < maxRetries && currentKey != nil; attempt++ {
		client, err := r.GetProviderClientWithKey(ctx, p, currentKey)
		if err != nil {
			lastErr = err
			currentKey, _ = r.SelectNextAPIKey(ctx, p.ID, currentKey.ID)
			continue
		}

		if err := fn(client); err != nil {
			lastErr = err
			r.logger.Warn("request failed, trying next API key",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.String("provider", p.Name),
			)
			if isQuotaOrRateLimitError(err.Error()) {
				r.MarkKeyFailed(currentKey.ID, err.Error())
			}
			currentKey, _ = r.SelectNextAPIKey(ctx, p.ID, currentKey.ID)
			continue
		}

		r.ClearKeyFailure(currentKey.ID)
		return currentKey, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("all API keys failed")
}

// EmbeddingResult contains the result of an ExecuteEmbeddings call.
type EmbeddingResult struct {
	Response *provider.EmbeddingResponse
	UsedKey  *models.ProviderAPIKey
}

// ExecuteEmbeddings sends an embedding request with automatic key-rotation retry.
func (r *Router) ExecuteEmbeddings(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.EmbeddingRequest, maxRetries int) (*EmbeddingResult, error) {
	var resp *provider.EmbeddingResponse
	usedKey, err := r.executeWithKeyRetry(ctx, p, apiKey, maxRetries, func(client provider.Client) error {
		var e error
		resp, e = client.Embeddings(ctx, req)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &EmbeddingResult{Response: resp, UsedKey: usedKey}, nil
}

// ImageResult contains the result of an ExecuteImage call.
type ImageResult struct {
	Response *provider.ImageGenerationResponse
	UsedKey  *models.ProviderAPIKey
}

// ExecuteImage sends an image generation request with automatic key-rotation retry.
func (r *Router) ExecuteImage(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.ImageGenerationRequest, maxRetries int) (*ImageResult, error) {
	var resp *provider.ImageGenerationResponse
	usedKey, err := r.executeWithKeyRetry(ctx, p, apiKey, maxRetries, func(client provider.Client) error {
		var e error
		resp, e = client.GenerateImage(ctx, req)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &ImageResult{Response: resp, UsedKey: usedKey}, nil
}

// AudioResult contains the result of an ExecuteAudio call.
type AudioResult struct {
	Response *provider.AudioTranscriptionResponse
	UsedKey  *models.ProviderAPIKey
}

// ExecuteAudio sends an audio transcription request with automatic key-rotation retry.
func (r *Router) ExecuteAudio(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.AudioTranscriptionRequest, maxRetries int) (*AudioResult, error) {
	var resp *provider.AudioTranscriptionResponse
	usedKey, err := r.executeWithKeyRetry(ctx, p, apiKey, maxRetries, func(client provider.Client) error {
		var e error
		resp, e = client.TranscribeAudio(ctx, req)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &AudioResult{Response: resp, UsedKey: usedKey}, nil
}

// SpeechResult contains the result of an ExecuteSpeech call.
type SpeechResult struct {
	Response *provider.SpeechResponse
	UsedKey  *models.ProviderAPIKey
}

// ExecuteSpeech sends a TTS request with automatic key-rotation retry.
func (r *Router) ExecuteSpeech(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.SpeechRequest, maxRetries int) (*SpeechResult, error) {
	var resp *provider.SpeechResponse
	usedKey, err := r.executeWithKeyRetry(ctx, p, apiKey, maxRetries, func(client provider.Client) error {
		var e error
		resp, e = client.SynthesizeSpeech(ctx, req)
		return e
	})
	if err != nil {
		return nil, err
	}
	return &SpeechResult{Response: resp, UsedKey: usedKey}, nil
}

// StreamResult contains the result of an ExecuteStreamChat call.
type StreamResult struct {
	Client  provider.Client
	Stream  <-chan provider.StreamChunk
	UsedKey *models.ProviderAPIKey
}

// ExecuteStreamChat obtains a streaming connection with automatic key-rotation retry.
// Retry is safe here because SSE headers have NOT yet been sent to the client.
// Once a stream channel is successfully obtained, it returns the client and stream for
// the handler to consume. After SSE headers are sent, retries are no longer possible.
func (r *Router) ExecuteStreamChat(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey, req *provider.ChatRequest, maxRetries int) (*StreamResult, error) {
	if !r.IsProviderHealthy(p.ID) {
		return nil, errors.New("provider is temporarily unavailable (circuit-breaker)")
	}

	// Phase 2: Inject MCP Tools
	r.injectMCPTools(ctx, req)

	if !p.RequiresAPIKey {
		client, err := r.GetProviderClientWithKey(ctx, p, nil)
		if err != nil {
			return nil, err
		}
		stream, err := client.StreamChat(ctx, req)
		if err != nil {
			if isProviderLevelError(err.Error()) {
				r.MarkProviderFailure(p.ID)
			}
			return nil, err
		}
		r.MarkProviderSuccess(p.ID)
		return &StreamResult{Client: client, Stream: stream}, nil
	}

	currentKey := apiKey
	var lastErr error

	for attempt := 0; attempt < maxRetries && currentKey != nil; attempt++ {
		client, err := r.GetProviderClientWithKey(ctx, p, currentKey)
		if err != nil {
			lastErr = err
			r.logger.Warn("stream: failed to create provider client, trying next key",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.String("provider", p.Name),
			)
			currentKey, _ = r.SelectNextAPIKey(ctx, p.ID, currentKey.ID)
			continue
		}

		stream, err := client.StreamChat(ctx, req)
		if err != nil {
			lastErr = err
			r.logger.Warn("stream: connection failed, trying next key",
				zap.Error(err),
				zap.Int("attempt", attempt+1),
				zap.String("provider", p.Name),
			)
			if isQuotaOrRateLimitError(err.Error()) {
				r.MarkKeyFailed(currentKey.ID, err.Error())
			} else if isProviderLevelError(err.Error()) {
				r.MarkProviderFailure(p.ID)
			}
			currentKey, _ = r.SelectNextAPIKey(ctx, p.ID, currentKey.ID)
			continue
		}

		r.ClearKeyFailure(currentKey.ID)
		r.MarkProviderSuccess(p.ID)
		return &StreamResult{Client: client, Stream: stream, UsedKey: currentKey}, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("all API keys failed for streaming")
}

// GetProviderClient returns the provider client from the registry.
func (r *Router) GetProviderClient(name string) (provider.Client, bool) {
	return r.registry.Get(name)
}

// GetProviderClientWithKey creates a provider client dynamically using the provided API key from database.
// This is the preferred method as API keys are stored encrypted in the database.
func (r *Router) GetProviderClientWithKey(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey) (provider.Client, error) {
	// For providers that don't require API keys
	if !p.RequiresAPIKey || apiKey == nil {
		// Try to get from registry first (for local providers like Ollama, LM Studio)
		if client, ok := r.registry.Get(p.Name); ok {
			return client, nil
		}
		// Create a client without API key
		cfg := &config.ProviderConfig{
			BaseURL:    p.BaseURL,
			HTTPClient: r.getHTTPClientProvider(ctx, p),
		}
		return r.createProviderClientWithRetry(p.Name, cfg, p.MaxRetries, p.Timeout)
	}

	// Decrypt the API key
	decryptedKey, err := crypto.Decrypt(apiKey.EncryptedAPIKey)
	if err != nil {
		return nil, errors.New("failed to decrypt API key")
	}

	cfg := &config.ProviderConfig{
		APIKey:     decryptedKey,
		BaseURL:    p.BaseURL,
		HTTPClient: r.getHTTPClientProvider(ctx, p),
	}

	return r.createProviderClientWithRetry(p.Name, cfg, p.MaxRetries, p.Timeout)
}

// getHTTPClientProvider returns a function that creates an HTTP client with optional proxy.
func (r *Router) getHTTPClientProvider(ctx context.Context, p *models.Provider) config.HTTPClientProvider {
	if !p.UseProxy {
		return nil
	}

	return func() *http.Client {
		var proxyInfo *models.Proxy

		// Use provider's default proxy if set
		if p.DefaultProxyID != nil {
			proxy, err := r.proxyRepo.GetByID(ctx, *p.DefaultProxyID)
			if err == nil && proxy.IsActive {
				proxyInfo = proxy
			}
		}

		// If no default proxy or it's inactive, get any active proxy
		if proxyInfo == nil {
			proxies, err := r.proxyRepo.GetActive(ctx)
			if err != nil || len(proxies) == 0 {
				// Return default client if no proxy available
				return &http.Client{Timeout: 600 * time.Second}
			}
			proxyInfo = &proxies[0]
		}

		proxyURL, err := url.Parse(proxyInfo.URL)
		if err != nil {
			return &http.Client{Timeout: 600 * time.Second}
		}

		// Add authentication if available
		if proxyInfo.Username != "" && proxyInfo.Password != "" {
			password, _ := crypto.Decrypt(proxyInfo.Password)
			proxyURL.User = url.UserPassword(proxyInfo.Username, password)
		}

		r.logger.Debug("using proxy for provider",
			zap.String("provider", p.Name),
			zap.String("proxy_url", proxyInfo.URL))

		transport := &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}

		return &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		}
	}
}

// createProviderClient creates a provider client based on provider name.
// Delegates to the shared factory in the provider package.
// Uses per-provider retry config when maxRetries > 0 or timeout > 0.
func (r *Router) createProviderClient(name string, cfg *config.ProviderConfig) (provider.Client, error) {
	return provider.NewClientByName(name, cfg, r.logger)
}

// createProviderClientWithRetry creates a provider client with per-provider retry overrides.
func (r *Router) createProviderClientWithRetry(name string, cfg *config.ProviderConfig, maxRetries, timeout int) (provider.Client, error) {
	retryCfg := provider.RetryConfigFromProvider(maxRetries, timeout)
	return provider.NewClientByNameWithRetry(name, cfg, retryCfg, r.logger)
}

// ─── Provider CRUD Operations ──────────────────────────────────────────────

// GetAllProviders returns all providers.
func (r *Router) GetAllProviders(ctx context.Context) ([]models.Provider, error) {
	return r.providerRepo.GetAll(ctx)
}

// GetProviderByID returns a provider by ID.
func (r *Router) GetProviderByID(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	return r.providerRepo.GetByID(ctx, id)
}

// GetProviderByName returns a provider by name.
func (r *Router) GetProviderByName(ctx context.Context, name string) (*models.Provider, error) {
	return r.providerRepo.GetByName(ctx, name)
}

// GetModelByID returns a model by ID.
func (r *Router) GetModelByID(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	return r.modelRepo.GetByID(ctx, id)
}

// UpdateProvider updates a provider.
func (r *Router) UpdateProvider(ctx context.Context, provider *models.Provider) error {
	return r.providerRepo.Update(ctx, provider)
}

// ToggleProviderAPIKey toggles a provider API key's active status.
func (r *Router) ToggleProviderAPIKey(ctx context.Context, id uuid.UUID) (*models.ProviderAPIKey, error) {
	key, err := r.providerKeyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	key.IsActive = !key.IsActive
	if err := r.providerKeyRepo.Update(ctx, key); err != nil {
		return nil, err
	}
	return key, nil
}

// GetAllProviderAPIKeys returns all API keys for a provider (including inactive).
func (r *Router) GetAllProviderAPIKeys(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	return r.providerKeyRepo.GetByProvider(ctx, providerID)
}

// GetProviderAPIKeys returns all API keys for a provider.
func (r *Router) GetProviderAPIKeys(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	return r.providerKeyRepo.GetActiveByProvider(ctx, providerID)
}

// CreateProviderAPIKey creates a new provider API key.
func (r *Router) CreateProviderAPIKey(ctx context.Context, key *models.ProviderAPIKey) error {
	return r.providerKeyRepo.Create(ctx, key)
}

// DeleteProviderAPIKey deletes a provider API key.
func (r *Router) DeleteProviderAPIKey(ctx context.Context, id uuid.UUID) error {
	return r.providerKeyRepo.Delete(ctx, id)
}

// UpdateProviderAPIKey updates a provider API key.
func (r *Router) UpdateProviderAPIKey(ctx context.Context, key *models.ProviderAPIKey) error {
	return r.providerKeyRepo.Update(ctx, key)
}

// GetProviderAPIKeyByID returns a provider API key by ID.
func (r *Router) GetProviderAPIKeyByID(ctx context.Context, id uuid.UUID) (*models.ProviderAPIKey, error) {
	return r.providerKeyRepo.GetByID(ctx, id)
}

// ─── Health Check ──────────────────────────────────────────────────────────

// HealthStatus represents provider health status.
type HealthStatus struct {
	ProviderID   uuid.UUID     `json:"provider_id"`
	ProviderName string        `json:"provider_name"`
	IsHealthy    bool          `json:"is_healthy"`
	Latency      time.Duration `json:"latency"`
	LastChecked  time.Time     `json:"last_checked"`
}

// CheckProviderHealth checks health of a specific provider.
func (r *Router) CheckProviderHealth(ctx context.Context, providerName string) (*HealthStatus, error) {
	// Get provider from database to check settings
	p, err := r.providerRepo.GetByName(ctx, providerName)
	if err != nil {
		return nil, errors.New("provider not found")
	}

	// First try to get from registry (for local providers like Ollama, LM Studio)
	client, ok := r.registry.Get(providerName)
	if !ok {
		if p.RequiresAPIKey {
			// Get an active API key for this provider
			apiKey, err := r.selectAPIKey(ctx, p.ID)
			if err != nil {
				return nil, errors.New("no active API keys for provider")
			}

			client, err = r.GetProviderClientWithKey(ctx, p, apiKey)
			if err != nil {
				return nil, err
			}
		} else {
			// Create client without API key
			cfg := &config.ProviderConfig{
				BaseURL: p.BaseURL,
			}
			client, err = r.createProviderClient(providerName, cfg)
			if err != nil {
				return nil, err
			}
		}
	}

	// If provider requires proxy, we need to use proxy for health check
	if p.UseProxy {
		r.logger.Info("provider requires proxy for health check", zap.String("provider", providerName))
	}

	healthy, latency, err := client.CheckHealth(ctx)
	return &HealthStatus{
		ProviderName: providerName,
		IsHealthy:    healthy,
		Latency:      latency,
		LastChecked:  time.Now(),
	}, err
}
