// Package router provides LLM request routing logic.
// This file contains provider CRUD operations, client creation, and health checks.
package router

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

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
	case "deepseek":
		return provider.NewDeepSeekClient(cfg, r.logger), nil
	case "mistral":
		return provider.NewMistralClient(cfg, r.logger), nil
	case "vllm":
		return provider.NewOpenAIClient(cfg, r.logger), nil
	default:
		// Default to OpenAI-compatible client
		return provider.NewOpenAIClient(cfg, r.logger), nil
	}
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
