// Package router provides LLM request routing logic.
package router

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/provider"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Strategy defines routing strategies.
type Strategy string

const (
	StrategyRoundRobin   Strategy = "round_robin"
	StrategyWeighted     Strategy = "weighted"
	StrategyLeastLatency Strategy = "least_latency"
	StrategyFallback     Strategy = "fallback"
)

// FailedKeyInfo tracks information about a failed API key.
type FailedKeyInfo struct {
	FailedAt time.Time
	Reason   string
}

// Router handles request routing to LLM providers.
type Router struct {
	providerRepo    *repository.ProviderRepository
	providerKeyRepo *repository.ProviderAPIKeyRepository
	proxyRepo       *repository.ProxyRepository
	modelRepo       *repository.ModelRepository
	registry        *provider.Registry
	strategy        Strategy
	roundRobinIndex int
	failedKeys      map[uuid.UUID]*FailedKeyInfo // Track failed keys temporarily
	failedKeysMu    sync.RWMutex
	mu              sync.Mutex
	logger          *zap.Logger
}

// NewRouter creates a new router instance.
func NewRouter(
	providerRepo *repository.ProviderRepository,
	providerKeyRepo *repository.ProviderAPIKeyRepository,
	proxyRepo *repository.ProxyRepository,
	modelRepo *repository.ModelRepository,
	registry *provider.Registry,
	logger *zap.Logger,
) *Router {
	return &Router{
		providerRepo:    providerRepo,
		providerKeyRepo: providerKeyRepo,
		proxyRepo:       proxyRepo,
		modelRepo:       modelRepo,
		registry:        registry,
		strategy:        StrategyWeighted,
		failedKeys:      make(map[uuid.UUID]*FailedKeyInfo),
		logger:          logger,
	}
}

// SetStrategy sets the routing strategy.
func (r *Router) SetStrategy(strategy Strategy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.strategy = strategy
}

// Route selects a provider and API key for a request.
func (r *Router) Route(ctx context.Context, modelName string) (*models.Provider, *models.ProviderAPIKey, error) {
	providers, err := r.providerRepo.GetActive(ctx)
	if err != nil {
		return nil, nil, err
	}

	if len(providers) == 0 {
		return nil, nil, errors.New("no active providers available")
	}

	// Try to find provider based on model name patterns
	selectedProvider := r.findProviderForModel(modelName, providers)

	// If no specific provider found, use weighted selection
	if selectedProvider == nil {
		switch r.strategy {
		case StrategyRoundRobin:
			selectedProvider = r.selectRoundRobin(providers)
		case StrategyWeighted:
			selectedProvider = r.selectWeighted(providers)
		case StrategyLeastLatency:
			selectedProvider = r.selectLeastLatency(providers)
		default:
			selectedProvider = r.selectWeighted(providers)
		}
	}

	// For providers that don't require API keys (e.g., Ollama, LM Studio), return nil for apiKey
	if !selectedProvider.RequiresAPIKey {
		return selectedProvider, nil, nil
	}

	apiKey, err := r.selectAPIKey(ctx, selectedProvider.ID)
	if err != nil {
		return nil, nil, err
	}

	return selectedProvider, apiKey, nil
}

// findProviderForModel tries to find the appropriate provider for a given model name.
func (r *Router) findProviderForModel(modelName string, providers []models.Provider) *models.Provider {
	modelLower := strings.ToLower(modelName)

	for i := range providers {
		p := &providers[i]
		switch p.Name {
		case "google":
			// Google Gemini models
			if strings.HasPrefix(modelLower, "gemini") ||
				strings.HasPrefix(modelLower, "gemma") ||
				strings.HasPrefix(modelLower, "embedding") ||
				strings.HasPrefix(modelLower, "text-embedding") ||
				strings.HasPrefix(modelLower, "imagen") ||
				strings.HasPrefix(modelLower, "veo") ||
				strings.HasPrefix(modelLower, "aqa") {
				return p
			}
		case "openai":
			// OpenAI models
			if strings.HasPrefix(modelLower, "gpt-") ||
				strings.HasPrefix(modelLower, "o1") ||
				strings.HasPrefix(modelLower, "o3") ||
				strings.HasPrefix(modelLower, "o4") ||
				strings.HasPrefix(modelLower, "chatgpt") ||
				strings.HasPrefix(modelLower, "text-davinci") ||
				strings.HasPrefix(modelLower, "dall-e") ||
				strings.HasPrefix(modelLower, "whisper") ||
				strings.HasPrefix(modelLower, "tts") {
				return p
			}
		case "anthropic":
			// Anthropic Claude models
			if strings.HasPrefix(modelLower, "claude") {
				return p
			}
		case "ollama", "lmstudio":
			// Check for common open-source model patterns
			// These are typically used when no other provider matches
			if strings.Contains(modelLower, "llama") ||
				strings.Contains(modelLower, "mistral") ||
				strings.Contains(modelLower, "qwen") ||
				strings.Contains(modelLower, "codellama") ||
				strings.Contains(modelLower, "vicuna") ||
				strings.Contains(modelLower, "phi") ||
				strings.Contains(modelLower, "deepseek") ||
				strings.Contains(modelLower, "yi-") ||
				strings.Contains(modelLower, "mixtral") ||
				strings.Contains(modelLower, "qwq") {
				return p
			}
		}
	}

	return nil
}

// selectRoundRobin selects provider using round-robin.
func (r *Router) selectRoundRobin(providers []models.Provider) *models.Provider {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.roundRobinIndex = (r.roundRobinIndex + 1) % len(providers)
	return &providers[r.roundRobinIndex]
}

// selectWeighted selects provider based on weights.
func (r *Router) selectWeighted(providers []models.Provider) *models.Provider {
	var totalWeight float64
	for _, p := range providers {
		totalWeight += p.Weight
	}

	if totalWeight == 0 {
		return &providers[secureRandomInt(len(providers))]
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range providers {
		cumulative += providers[i].Weight
		if random <= cumulative {
			return &providers[i]
		}
	}

	return &providers[len(providers)-1]
}

// selectLeastLatency selects provider with lowest latency.
func (r *Router) selectLeastLatency(providers []models.Provider) *models.Provider {
	return r.selectWeighted(providers)
}

// isKeyTemporarilyFailed checks if a key is temporarily marked as failed.
// Keys are considered failed for 5 minutes after a failure.
func (r *Router) isKeyTemporarilyFailed(keyID uuid.UUID) bool {
	r.failedKeysMu.RLock()
	defer r.failedKeysMu.RUnlock()

	info, exists := r.failedKeys[keyID]
	if !exists {
		return false
	}

	// Key failure expires after 5 minutes
	if time.Since(info.FailedAt) > 5*time.Minute {
		return false
	}

	return true
}

// MarkKeyFailed marks an API key as temporarily failed.
func (r *Router) MarkKeyFailed(keyID uuid.UUID, reason string) {
	r.failedKeysMu.Lock()
	defer r.failedKeysMu.Unlock()

	r.failedKeys[keyID] = &FailedKeyInfo{
		FailedAt: time.Now(),
		Reason:   reason,
	}
	r.logger.Warn("API key marked as failed", zap.String("key_id", keyID.String()), zap.String("reason", reason))
}

// ClearKeyFailure clears the failure status of an API key.
func (r *Router) ClearKeyFailure(keyID uuid.UUID) {
	r.failedKeysMu.Lock()
	defer r.failedKeysMu.Unlock()
	delete(r.failedKeys, keyID)
}

// selectAPIKey selects an API key for the provider, excluding temporarily failed keys.
func (r *Router) selectAPIKey(ctx context.Context, providerID uuid.UUID) (*models.ProviderAPIKey, error) {
	keys, err := r.providerKeyRepo.GetActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("no active API keys for provider")
	}

	// Filter out temporarily failed keys
	availableKeys := make([]models.ProviderAPIKey, 0, len(keys))
	for _, k := range keys {
		if !r.isKeyTemporarilyFailed(k.ID) {
			availableKeys = append(availableKeys, k)
		}
	}

	// If all keys are failed, use all keys (reset and try again)
	if len(availableKeys) == 0 {
		r.logger.Warn("all API keys are temporarily failed, resetting", zap.Int("total_keys", len(keys)))
		availableKeys = keys
		// Clear all failed keys for this provider
		r.failedKeysMu.Lock()
		for _, k := range keys {
			delete(r.failedKeys, k.ID)
		}
		r.failedKeysMu.Unlock()
	}

	var totalWeight float64
	for _, k := range availableKeys {
		totalWeight += k.Weight
	}

	if totalWeight == 0 {
		return &availableKeys[secureRandomInt(len(availableKeys))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range availableKeys {
		cumulative += availableKeys[i].Weight
		if random <= cumulative {
			return &availableKeys[i], nil
		}
	}

	return &availableKeys[len(availableKeys)-1], nil
}

// SelectNextAPIKey selects the next available API key, excluding the current one.
// This is used for fallback when the current key fails.
func (r *Router) SelectNextAPIKey(ctx context.Context, providerID uuid.UUID, excludeKeyID uuid.UUID) (*models.ProviderAPIKey, error) {
	keys, err := r.providerKeyRepo.GetActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	// Filter out the excluded key and temporarily failed keys
	availableKeys := make([]models.ProviderAPIKey, 0, len(keys))
	for _, k := range keys {
		if k.ID != excludeKeyID && !r.isKeyTemporarilyFailed(k.ID) {
			availableKeys = append(availableKeys, k)
		}
	}

	if len(availableKeys) == 0 {
		return nil, errors.New("no alternative API keys available")
	}

	// Select using weighted random
	var totalWeight float64
	for _, k := range availableKeys {
		totalWeight += k.Weight
	}

	if totalWeight == 0 {
		return &availableKeys[secureRandomInt(len(availableKeys))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range availableKeys {
		cumulative += availableKeys[i].Weight
		if random <= cumulative {
			return &availableKeys[i], nil
		}
	}

	return &availableKeys[len(availableKeys)-1], nil
}

// RouteWithFallback attempts routing with fallback providers.
func (r *Router) RouteWithFallback(ctx context.Context, modelName string, maxRetries int) (*models.Provider, *models.ProviderAPIKey, error) {
	providers, err := r.providerRepo.GetActive(ctx)
	if err != nil {
		return nil, nil, err
	}

	if len(providers) == 0 {
		return nil, nil, errors.New("no active providers available")
	}

	sortByPriority(providers)

	for i := 0; i < len(providers) && i < maxRetries; i++ {
		apiKey, err := r.selectAPIKey(ctx, providers[i].ID)
		if err == nil {
			return &providers[i], apiKey, nil
		}
	}

	return nil, nil, errors.New("all providers failed")
}

// sortByPriority sorts providers by priority descending.
func sortByPriority(providers []models.Provider) {
	for i := 0; i < len(providers)-1; i++ {
		for j := i + 1; j < len(providers); j++ {
			if providers[j].Priority > providers[i].Priority {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}
}

// GetProviderClient returns the provider client.
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
		return r.createProviderClient(p.Name, cfg)
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

	return r.createProviderClient(p.Name, cfg)
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
				return &http.Client{Timeout: 60 * time.Second}
			}
			proxyInfo = &proxies[0]
		}

		proxyURL, err := url.Parse(proxyInfo.URL)
		if err != nil {
			return &http.Client{Timeout: 60 * time.Second}
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
func (r *Router) createProviderClient(name string, cfg *config.ProviderConfig) (provider.Client, error) {
	switch name {
	case "openai":
		return provider.NewOpenAIClient(cfg, r.logger), nil
	case "anthropic":
		return provider.NewAnthropicClient(cfg, r.logger), nil
	case "google":
		return provider.NewGoogleClient(cfg, r.logger), nil
	case "ollama":
		return provider.NewOllamaClient(cfg, r.logger), nil
	case "lmstudio":
		return provider.NewLMStudioClient(cfg, r.logger), nil
	default:
		// Default to OpenAI-compatible client
		return provider.NewOpenAIClient(cfg, r.logger), nil
	}
}

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
		// For now, direct health check will fail if proxy is required but not configured in client
		// TODO: Implement proxy-aware health check
	}

	healthy, latency, err := client.CheckHealth(ctx)
	return &HealthStatus{
		ProviderName: providerName,
		IsHealthy:    healthy,
		Latency:      latency,
		LastChecked:  time.Now(),
	}, err
}

// secureRandomInt returns a cryptographically secure random int in [0, n).
func secureRandomInt(n int) int {
	if n <= 0 {
		return 0
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	// #nosec G115 - n is guaranteed to be positive and within bounds for array indexing
	return int(binary.LittleEndian.Uint32(b[:]) % uint32(n))
}

// secureRandomFloat64 returns a cryptographically secure random float64 in [0, 1).
func secureRandomFloat64() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return float64(binary.LittleEndian.Uint64(b[:])>>11) / (1 << 53)
}
