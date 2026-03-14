// Package router provides LLM request routing logic.
// This file contains the core Router struct, Route method, and API key management.
package router

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
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
	StrategyRoundRobin    Strategy = "round_robin"
	StrategyWeighted      Strategy = "weighted"
	StrategyLeastLatency  Strategy = "least_latency"
	StrategyFallback      Strategy = "fallback"
	StrategyCostOptimized Strategy = "cost_optimized"
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
		case StrategyCostOptimized:
			selectedProvider = r.selectCostOptimized(ctx, modelName, providers)
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

// ─── API Key Management ────────────────────────────────────────────────────

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

	// Find the best (lowest) priority among available keys
	bestPriority := math.MaxInt32
	for _, k := range availableKeys {
		if k.Priority < bestPriority && k.Priority > 0 {
			bestPriority = k.Priority
		} else if k.Priority == 0 && 1 < bestPriority {
			bestPriority = 1 // default priority if 0
		}
	}

	// Filter keys down to just the ones with the best priority
	var priorityKeys []models.ProviderAPIKey
	for _, k := range availableKeys {
		prio := k.Priority
		if prio == 0 {
			prio = 1
		}
		if prio == bestPriority {
			priorityKeys = append(priorityKeys, k)
		}
	}

	var totalWeight float64
	for _, k := range priorityKeys {
		totalWeight += k.Weight
	}

	if totalWeight == 0 {
		return &priorityKeys[secureRandomInt(len(priorityKeys))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range priorityKeys {
		cumulative += priorityKeys[i].Weight
		if random <= cumulative {
			return &priorityKeys[i], nil
		}
	}

	return &priorityKeys[len(priorityKeys)-1], nil
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

	bestPriority := math.MaxInt32
	for _, k := range availableKeys {
		if k.Priority < bestPriority && k.Priority > 0 {
			bestPriority = k.Priority
		} else if k.Priority == 0 && 1 < bestPriority {
			bestPriority = 1
		}
	}

	var priorityKeys []models.ProviderAPIKey
	for _, k := range availableKeys {
		prio := k.Priority
		if prio == 0 {
			prio = 1
		}
		if prio == bestPriority {
			priorityKeys = append(priorityKeys, k)
		}
	}

	// Select using weighted random
	var totalWeight float64
	for _, k := range priorityKeys {
		totalWeight += k.Weight
	}

	if totalWeight == 0 {
		return &priorityKeys[secureRandomInt(len(priorityKeys))], nil
	}

	random := secureRandomFloat64() * totalWeight
	var cumulative float64
	for i := range priorityKeys {
		cumulative += priorityKeys[i].Weight
		if random <= cumulative {
			return &priorityKeys[i], nil
		}
	}

	return &priorityKeys[len(priorityKeys)-1], nil
}

// ─── Cryptographic Random Utilities ────────────────────────────────────────

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
