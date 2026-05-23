// Package router provides LLM request routing logic.
// This file contains provider CRUD operations, client creation, and health checks.
package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/pkg/sanitize"

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
		if err != nil && isProviderLevelError(err) {
			r.MarkProviderFailure(p.ID)
		} else if err == nil {
			r.MarkProviderSuccess(p.ID)
		}
		return res, err
	}

	currentKey := apiKey
	if currentKey == nil {
		var err error
		currentKey, err = r.selectAPIKey(ctx, p.ID)
		if err != nil {
			return nil, err
		}
	}
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
		if isQuotaOrRateLimitError(err) {
			r.MarkKeyFailed(currentKey.ID, err.Error())
		} else if isProviderLevelError(err) {
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

		var args map[string]json.RawMessage
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

// isProviderLevelError checks if an error should trigger provider circuit
// breaking. The canonical signal is the upstream HTTP status carried on a
// *provider.ProviderError (5xx → trip the breaker). When that's unavailable
// (network errors, client-side wrapping) we fall back to substring matching
// on the message.
func isProviderLevelError(err error) bool {
	if err == nil {
		return false
	}
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		return pe.StatusCode >= 500 && pe.StatusCode < 600
	}
	return messageLooksProviderLevel(err.Error())
}

// messageLooksProviderLevel keeps the legacy substring heuristic for callers
// that have no typed error (network failures from net/http, context errors).
func messageLooksProviderLevel(errMsg string) bool {
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

// isQuotaOrRateLimitError checks if the error indicates a quota or
// rate-limit problem that should trigger an API-key rotation. The canonical
// signal is HTTP 429 on a *provider.ProviderError. Some providers also
// reject over-budget keys with 402/403 — we treat those the same. Falls back
// to substring matching when no typed error is available.
func isQuotaOrRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	var pe *provider.ProviderError
	if errors.As(err, &pe) {
		switch pe.StatusCode {
		case http.StatusTooManyRequests, http.StatusPaymentRequired:
			return true
		case http.StatusForbidden:
			// 403 is overloaded: auth failures vs quota exhaustion. Disambiguate
			// by body if present, otherwise treat as auth (don't rotate).
			return messageLooksQuotaLimited(string(pe.Body)) || messageLooksQuotaLimited(pe.Message)
		}
		return false
	}
	return messageLooksQuotaLimited(err.Error())
}

// messageLooksQuotaLimited is the legacy substring heuristic, retained as a
// fallback for non-typed errors and exposed for the unit test that pins the
// keyword list.
func messageLooksQuotaLimited(errMsg string) bool {
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
	if currentKey == nil {
		var err error
		currentKey, err = r.selectAPIKey(ctx, p.ID)
		if err != nil {
			return nil, err
		}
	}
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
			if isQuotaOrRateLimitError(err) {
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
			if isProviderLevelError(err) {
				r.MarkProviderFailure(p.ID)
			}
			return nil, err
		}
		r.MarkProviderSuccess(p.ID)
		return &StreamResult{Client: client, Stream: stream}, nil
	}

	currentKey := apiKey
	if currentKey == nil {
		var err error
		currentKey, err = r.selectAPIKey(ctx, p.ID)
		if err != nil {
			return nil, err
		}
	}
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
			if isQuotaOrRateLimitError(err) {
				r.MarkKeyFailed(currentKey.ID, err.Error())
			} else if isProviderLevelError(err) {
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
	proxyInfo, err := r.resolveProxyForRequest(ctx, p, apiKey)
	if err != nil {
		return nil, err
	}
	httpClientProvider, err := r.getHTTPClientProvider(p, proxyInfo)
	if err != nil {
		return nil, err
	}

	// For providers that don't require API keys
	if !p.RequiresAPIKey || apiKey == nil {
		// Try to get from registry first (for local providers like Ollama, LM Studio)
		if client, ok := r.registry.Get(p.Name); ok {
			return client, nil
		}
		// Create a client without API key
		cfg := &config.ProviderConfig{
			BaseURL:    p.BaseURL,
			HTTPClient: httpClientProvider,
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
		HTTPClient: httpClientProvider,
	}

	return r.createProviderClientWithRetry(p.Name, cfg, p.MaxRetries, p.Timeout)
}

// getHTTPClientProvider returns a function that creates an HTTP client with
// SSRF dial-time protection, plus optional proxy when the provider is so
// configured. Always returns a non-nil provider so every provider client
// picks up SafeTransport — never a bare &http.Client{}.
func (r *Router) getHTTPClientProvider(p *models.Provider, proxyInfo *models.Proxy) (config.HTTPClientProvider, error) {
	if proxyInfo == nil {
		return func() *http.Client {
			return sanitize.SafeHTTPClient(allowLocalProviderEgress, 600*time.Second)
		}, nil
	}

	proxyURL, err := r.proxyURL(proxyInfo)
	if err != nil {
		return nil, err
	}

	return func() *http.Client {
		r.logger.Debug("using proxy for upstream provider request",
			zap.String("provider", p.Name),
			zap.String("proxy_id", proxyInfo.ID.String()),
			zap.String("proxy_url", proxyInfo.URL))

		return sanitize.SafeHTTPClientWithProxy(allowLocalProviderEgress, 60*time.Second, proxyURL)
	}, nil
}

// resolveProxyForRequest picks the effective proxy for a provider/API-key pair.
// Explicit account bindings fail closed; provider-level proxy mode falls back to
// another active proxy, but never to direct egress.
func (r *Router) resolveProxyForRequest(ctx context.Context, p *models.Provider, apiKey *models.ProviderAPIKey) (*models.Proxy, error) {
	if apiKey != nil {
		if apiKey.ProxyID != nil {
			proxyInfo, err := r.proxyRepo.GetByID(ctx, *apiKey.ProxyID)
			if err != nil {
				return nil, fmt.Errorf("bound proxy unavailable: %w", err)
			}
			if !proxyInfo.IsActive {
				return nil, fmt.Errorf("bound proxy is inactive")
			}
			return proxyInfo, nil
		}
		if apiKey.ProxyPoolID != nil {
			pool, err := r.proxyRepo.GetPoolByID(ctx, *apiKey.ProxyPoolID)
			if err != nil {
				return nil, fmt.Errorf("bound proxy pool unavailable: %w", err)
			}
			if !pool.IsActive {
				return nil, fmt.Errorf("bound proxy pool is inactive")
			}
			proxies, err := r.proxyRepo.GetActiveByPoolID(ctx, *apiKey.ProxyPoolID)
			if err != nil {
				return nil, fmt.Errorf("failed to load bound proxy pool: %w", err)
			}
			proxyInfo := selectWeightedProxy(proxies)
			if proxyInfo == nil {
				return nil, fmt.Errorf("bound proxy pool has no active proxies")
			}
			return proxyInfo, nil
		}
	}

	if !p.UseProxy {
		return nil, nil
	}

	if p.DefaultProxyID != nil {
		proxyInfo, err := r.proxyRepo.GetByID(ctx, *p.DefaultProxyID)
		if err == nil && proxyInfo.IsActive {
			return proxyInfo, nil
		}
		r.logger.Warn("provider default proxy unavailable, trying active proxy pool",
			zap.String("provider", p.Name),
			zap.String("proxy_id", p.DefaultProxyID.String()),
			zap.Error(err))
	}

	proxies, err := r.proxyRepo.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load active proxies: %w", err)
	}
	proxyInfo := selectWeightedProxy(proxies)
	if proxyInfo == nil {
		return nil, fmt.Errorf("provider requires proxy but no active proxy is available")
	}
	return proxyInfo, nil
}

func (r *Router) proxyURL(proxyInfo *models.Proxy) (*url.URL, error) {
	proxyURL, err := url.Parse(normalizeProxyURL(proxyInfo))
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	if proxyInfo.Username != "" && proxyInfo.Password != "" {
		password, decErr := crypto.Decrypt(proxyInfo.Password)
		if decErr != nil {
			return nil, fmt.Errorf("proxy password decryption failed: %w", decErr)
		}
		proxyURL.User = url.UserPassword(proxyInfo.Username, password)
	}

	return proxyURL, nil
}

func normalizeProxyURL(proxyInfo *models.Proxy) string {
	if strings.Contains(proxyInfo.URL, "://") {
		return proxyInfo.URL
	}
	switch proxyInfo.Type {
	case "socks5":
		return "socks5://" + proxyInfo.URL
	case "https":
		return "https://" + proxyInfo.URL
	default:
		return "http://" + proxyInfo.URL
	}
}

func selectWeightedProxy(proxies []models.Proxy) *models.Proxy {
	if len(proxies) == 0 {
		return nil
	}
	var totalWeight float64
	for _, p := range proxies {
		totalWeight += p.Weight
	}
	if totalWeight <= 0 {
		return &proxies[secureRandomInt(len(proxies))]
	}
	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range proxies {
		cumulative += proxies[i].Weight
		if random <= cumulative {
			return &proxies[i]
		}
	}
	return &proxies[len(proxies)-1]
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

// CreateModel creates a new model for a provider.
func (r *Router) CreateModel(ctx context.Context, m *models.Model) error {
	return r.modelRepo.Create(ctx, m)
}

// UpdateModel updates an existing model.
func (r *Router) UpdateModel(ctx context.Context, m *models.Model) error {
	return r.modelRepo.Update(ctx, m)
}

// DeleteModel deletes a model by ID.
func (r *Router) DeleteModel(ctx context.Context, id uuid.UUID) error {
	return r.modelRepo.Delete(ctx, id)
}

// ToggleModel toggles a model's active status.
func (r *Router) ToggleModel(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	m, err := r.modelRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	m.IsActive = !m.IsActive
	if err := r.modelRepo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetModelsByProvider returns all models for a provider, sorted by name.
func (r *Router) GetModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]models.Model, error) {
	return r.modelRepo.GetByProviderSorted(ctx, providerID)
}

// SyncModelsResult contains the result of a SyncProviderModels call.
type SyncModelsResult struct {
	NewModels []models.Model
	Total     int
}

// SyncProviderModels discovers models through the provider client implementation
// and upserts any new ones. Returns all models for the provider.
func (r *Router) SyncProviderModels(ctx context.Context, providerID uuid.UUID) ([]models.Model, error) {
	prov, err := r.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return nil, errors.New("provider not found")
	}

	// Load existing models
	existingModels, err := r.modelRepo.GetByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	// Get first active API key for auth
	apiKeys, err := r.providerKeyRepo.GetActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if prov.RequiresAPIKey && len(apiKeys) == 0 {
		return nil, errors.New("provider requires an active API key")
	}

	var apiKey *models.ProviderAPIKey
	if len(apiKeys) > 0 {
		apiKey = &apiKeys[0]
	}

	client, err := r.GetProviderClientWithKey(ctx, prov, apiKey)
	if err != nil {
		return nil, fmt.Errorf("create provider client: %w", err)
	}

	upstreamModels, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list provider models: %w", err)
	}

	// Build existing model map
	existing := make(map[string]bool)
	for _, m := range existingModels {
		existing[m.Name] = true
	}

	// Upsert discovered models
	for _, upstream := range upstreamModels {
		modelName := upstream.ID
		if modelName == "" {
			modelName = upstream.Name
		}
		if modelName == "" || existing[modelName] {
			continue
		}
		m := models.Model{
			ProviderID:  providerID,
			Name:        modelName,
			DisplayName: modelName,
			IsActive:    true,
			MaxTokens:   4096,
		}
		if err := r.modelRepo.Create(ctx, &m); err != nil {
			return nil, fmt.Errorf("create model %q: %w", modelName, err)
		}
		existing[modelName] = true
	}

	// Return full model list
	return r.modelRepo.GetByProviderSorted(ctx, providerID)
}

// CreateProvider creates a new LLM provider.
func (r *Router) CreateProvider(ctx context.Context, provider *models.Provider) error {
	return r.providerRepo.Create(ctx, provider)
}

// UpdateProvider updates a provider.
func (r *Router) UpdateProvider(ctx context.Context, provider *models.Provider) error {
	return r.providerRepo.Update(ctx, provider)
}

// DeleteProvider removes a provider by ID.
func (r *Router) DeleteProvider(ctx context.Context, id uuid.UUID) error {
	return r.providerRepo.Delete(ctx, id)
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
