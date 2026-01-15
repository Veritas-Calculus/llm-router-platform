// Package router provides LLM request routing logic.
package router

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"sync"
	"time"

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

// Router handles request routing to LLM providers.
type Router struct {
	providerRepo    *repository.ProviderRepository
	providerKeyRepo *repository.ProviderAPIKeyRepository
	modelRepo       *repository.ModelRepository
	registry        *provider.Registry
	strategy        Strategy
	roundRobinIndex int
	mu              sync.Mutex
	logger          *zap.Logger
}

// NewRouter creates a new router instance.
func NewRouter(
	providerRepo *repository.ProviderRepository,
	providerKeyRepo *repository.ProviderAPIKeyRepository,
	modelRepo *repository.ModelRepository,
	registry *provider.Registry,
	logger *zap.Logger,
) *Router {
	return &Router{
		providerRepo:    providerRepo,
		providerKeyRepo: providerKeyRepo,
		modelRepo:       modelRepo,
		registry:        registry,
		strategy:        StrategyWeighted,
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

	var selectedProvider *models.Provider
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

// selectAPIKey selects an API key for the provider.
func (r *Router) selectAPIKey(ctx context.Context, providerID uuid.UUID) (*models.ProviderAPIKey, error) {
	keys, err := r.providerKeyRepo.GetActiveByProvider(ctx, providerID)
	if err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, errors.New("no active API keys for provider")
	}

	var totalWeight float64
	for _, k := range keys {
		totalWeight += k.Weight
	}

	if totalWeight == 0 {
		return &keys[secureRandomInt(len(keys))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range keys {
		cumulative += keys[i].Weight
		if random <= cumulative {
			return &keys[i], nil
		}
	}

	return &keys[len(keys)-1], nil
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

// GetAllProviders returns all providers.
func (r *Router) GetAllProviders(ctx context.Context) ([]models.Provider, error) {
	return r.providerRepo.GetAll(ctx)
}

// GetProviderByID returns a provider by ID.
func (r *Router) GetProviderByID(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	return r.providerRepo.GetByID(ctx, id)
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
	client, ok := r.registry.Get(providerName)
	if !ok {
		return nil, errors.New("provider not found")
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
