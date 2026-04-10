// Package router provides LLM request routing logic.
// This file contains the core Router struct, Route method, and API key management.
package router

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/internal/service/mcp"
	"llm-router-platform/internal/service/provider"

	"github.com/redis/go-redis/v9"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
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

// modelDiscoveryCache caches discovered model→provider mappings.
type modelDiscoveryCache struct {
	// modelToProvider maps model name (lowercase) → provider name.
	modelToProvider map[string]string
	fetchedAt       time.Time
}

// modelProviderCache caches DB model→provider-index mappings.
type modelProviderCache struct {
	// modelToProviderIdx maps model name (lowercase) → index into providers slice.
	modelToProviderIdx map[string]int
	fetchedAt          time.Time
}

const (
	// failedKeyTTL is how long a key failure is remembered.
	failedKeyTTL = 5 * time.Minute
	// failedKeyPrefix is the Redis key prefix for failed API keys.
	failedKeyPrefix = "router:failed_key:"
	// cacheTTL is the TTL for model caches.
	cacheTTL = 5 * time.Minute
)

// Router handles request routing to LLM providers.
type Router struct {
	providerRepo     repository.ProviderRepo
	providerKeyRepo  repository.ProviderAPIKeyRepo
	proxyRepo        repository.ProxyRepo
	modelRepo        repository.ModelRepo
	routingRuleRepo  repository.RoutingRuleRepo
	registry         *provider.Registry
	mcpService       *mcp.Service
	strategy         Strategy
	roundRobinIndex  int
	redisClient      *redis.Client          // nil = use in-memory fallback
	failedKeys       map[uuid.UUID]*FailedKeyInfo // In-memory fallback when Redis unavailable
	failedKeysMu     sync.RWMutex
	providerLatency  map[uuid.UUID]int64    // EWMA latency per provider (ms)
	latencyMu        sync.RWMutex
	modelCache       *modelProviderCache    // Cached DB model→provider map
	modelCacheMu     sync.RWMutex
	mu               sync.Mutex
	discoveryCache   *modelDiscoveryCache
	discoveryCacheMu sync.RWMutex
	cacheSF          singleflight.Group      // Dedup concurrent model-provider cache refreshes
	circuitBreaker   *CircuitBreaker         // Provider-level circuit breaker (3-state)
	retryCfg         RetryConfig             // Exponential backoff config
	logger           *zap.Logger
}

// NewRouter creates a new router instance.
func NewRouter(
	providerRepo repository.ProviderRepo,
	providerKeyRepo repository.ProviderAPIKeyRepo,
	proxyRepo repository.ProxyRepo,
	modelRepo repository.ModelRepo,
	routingRuleRepo repository.RoutingRuleRepo,
	registry *provider.Registry,
	mcpService *mcp.Service,
	logger *zap.Logger,
) *Router {
	return &Router{
		providerRepo:    providerRepo,
		providerKeyRepo: providerKeyRepo,
		proxyRepo:       proxyRepo,
		modelRepo:       modelRepo,
		routingRuleRepo: routingRuleRepo,
		registry:        registry,
		mcpService:      mcpService,
		strategy:        StrategyWeighted,
		failedKeys:      make(map[uuid.UUID]*FailedKeyInfo),
		circuitBreaker:  NewCircuitBreaker(DefaultCircuitBreakerConfig(), logger),
		retryCfg:        DefaultRetryConfig(),
		logger:          logger,
	}
}

// ─── Provider Circuit Breaking (delegated to CircuitBreaker) ───────────────

// IsProviderHealthy checks if a provider's circuit is allowing requests.
func (r *Router) IsProviderHealthy(providerID uuid.UUID) bool {
	return r.circuitBreaker.AllowRequest(providerID)
}

// MarkProviderSuccess records a successful request for the provider's circuit breaker.
func (r *Router) MarkProviderSuccess(providerID uuid.UUID) {
	// Look up provider name for metrics labels (best-effort)
	name := r.resolveProviderName(providerID)
	r.circuitBreaker.RecordSuccess(providerID, name)
}

// MarkProviderFailure records a failed request for the provider's circuit breaker.
func (r *Router) MarkProviderFailure(providerID uuid.UUID) {
	name := r.resolveProviderName(providerID)
	r.circuitBreaker.RecordFailure(providerID, name)
}

// ResetProviderCircuit manually resets a provider's circuit breaker to closed state.
func (r *Router) ResetProviderCircuit(providerID uuid.UUID) {
	r.circuitBreaker.Reset(providerID)
}

// GetProviderCircuitState returns the current circuit breaker state for a provider.
func (r *Router) GetProviderCircuitState(providerID uuid.UUID) (CircuitState, int) {
	return r.circuitBreaker.GetState(providerID)
}

// resolveProviderName does a best-effort lookup of a provider's name by ID.
// Used for Prometheus labels — must not block on DB.
func (r *Router) resolveProviderName(providerID uuid.UUID) string {
	p, err := r.providerRepo.GetByID(context.Background(), providerID)
	if err != nil {
		return providerID.String()
	}
	return p.Name
}

// SetRedisClient sets the Redis client for cross-instance key failure sharing.
func (r *Router) SetRedisClient(client *redis.Client) {
	r.redisClient = client
}

// getModelProviderCache returns a cached map of model name (lowercase) → provider index.
// Refreshes from DB every 5 minutes. Uses singleflight to prevent thundering herd
// when multiple goroutines hit an expired cache simultaneously.
func (r *Router) getModelProviderCache(providers []models.Provider) map[string]int {
	r.modelCacheMu.RLock()
	if r.modelCache != nil && time.Since(r.modelCache.fetchedAt) < cacheTTL {
		result := r.modelCache.modelToProviderIdx
		r.modelCacheMu.RUnlock()
		return result
	}
	r.modelCacheMu.RUnlock()

	v, _, _ := r.cacheSF.Do("model-provider-cache", func() (interface{}, error) {
		// Double-check: another goroutine may have refreshed while we waited.
		r.modelCacheMu.RLock()
		if r.modelCache != nil && time.Since(r.modelCache.fetchedAt) < cacheTTL {
			cached := r.modelCache.modelToProviderIdx
			r.modelCacheMu.RUnlock()
			return cached, nil
		}
		r.modelCacheMu.RUnlock()

		result := make(map[string]int)
		for i := range providers {
			dbModels, err := r.modelRepo.GetByProvider(context.Background(), providers[i].ID)
			if err != nil {
				continue
			}
			for _, m := range dbModels {
				if m.IsActive {
					result[strings.ToLower(m.Name)] = i
				}
			}
		}

		r.modelCacheMu.Lock()
		r.modelCache = &modelProviderCache{
			modelToProviderIdx: result,
			fetchedAt:          time.Now(),
		}
		r.modelCacheMu.Unlock()

		r.logger.Debug("model-provider cache refreshed", zap.Int("models_cached", len(result)))
		return result, nil
	})

	return v.(map[string]int)
}

// getDiscoveryCache returns the cached model→provider map if still valid.
func (r *Router) getDiscoveryCache() map[string]string {
	r.discoveryCacheMu.RLock()
	defer r.discoveryCacheMu.RUnlock()
	if r.discoveryCache == nil || time.Since(r.discoveryCache.fetchedAt) > 5*time.Minute {
		return nil
	}
	return r.discoveryCache.modelToProvider
}

// refreshDiscoveryCache rebuilds the model→provider cache by querying upstreams.
func (r *Router) refreshDiscoveryCache(providers []models.Provider) map[string]string {
	result := make(map[string]string)
	for i := range providers {
		p := &providers[i]
		client, ok := r.registry.Get(p.Name)
		if !ok && !p.RequiresAPIKey {
			cfg := &config.ProviderConfig{BaseURL: p.BaseURL}
			var err error
			client, err = r.createProviderClient(p.Name, cfg)
			if err != nil || client == nil {
				continue
			}
		} else if !ok {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		upstreamModels, err := client.ListModels(ctx)
		cancel()
		if err != nil {
			continue
		}
		for _, m := range upstreamModels {
			result[strings.ToLower(m.ID)] = p.Name
		}
	}

	r.discoveryCacheMu.Lock()
	r.discoveryCache = &modelDiscoveryCache{
		modelToProvider: result,
		fetchedAt:       time.Now(),
	}
	r.discoveryCacheMu.Unlock()

	r.logger.Debug("model discovery cache refreshed", zap.Int("models_found", len(result)))
	return result
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

	// 1. Evaluate explicit Routing Rules
	selectedProvider = r.evaluateRoutingRules(ctx, modelName, providers)

	// 2. Try to find provider based on model name patterns (Heuristics)
	if selectedProvider == nil {
		selectedProvider = r.findProviderForModel(modelName, providers)
	}

	// 3. If no specific provider found, use strategy selection
	if selectedProvider == nil {
		selectedProvider = r.selectByStrategy(ctx, modelName, providers)
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

// evaluateRoutingRules checks explicit routing rules and returns a matching provider, or nil.
func (r *Router) evaluateRoutingRules(ctx context.Context, modelName string, providers []models.Provider) *models.Provider {
	rules, err := r.routingRuleRepo.GetActive(ctx)
	if err != nil || len(rules) == 0 {
		return nil
	}

	// Sort by Priority DESC, CreatedAt ASC
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return rules[i].CreatedAt.Before(rules[j].CreatedAt)
	})

	for _, rule := range rules {
		if !matchesGlobPattern(modelName, rule.ModelPattern) {
			continue
		}

		// Try to find the target provider
		if p := r.findHealthyProvider(rule.TargetProviderID, providers); p != nil {
			return p
		}

		// Try fallback provider
		if rule.FallbackProviderID != nil {
			if p := r.findHealthyProvider(*rule.FallbackProviderID, providers); p != nil {
				return p
			}
		}

		// Matched a rule but both providers unavailable -- stop evaluating
		if rule.FallbackProviderID != nil {
			break
		}
	}
	return nil
}

// findHealthyProvider returns the provider with the given ID if it exists and its circuit is not open.
func (r *Router) findHealthyProvider(providerID uuid.UUID, providers []models.Provider) *models.Provider {
	for i := range providers {
		if providers[i].ID != providerID {
			continue
		}
		if r.circuitBreaker.AllowRequest(providers[i].ID) {
			return &providers[i]
		}
		return nil
	}
	return nil
}

// selectByStrategy selects a provider based on the configured routing strategy.
func (r *Router) selectByStrategy(ctx context.Context, modelName string, providers []models.Provider) *models.Provider {
	switch r.strategy {
	case StrategyRoundRobin:
		return r.selectRoundRobin(providers)
	case StrategyWeighted:
		return r.selectWeighted(providers)
	case StrategyLeastLatency:
		return r.selectLeastLatency(providers)
	case StrategyCostOptimized:
		return r.selectCostOptimized(ctx, modelName, providers)
	default:
		return r.selectWeighted(providers)
	}
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
// Uses Redis when available (cross-instance), falls back to in-memory map.
func (r *Router) isKeyTemporarilyFailed(keyID uuid.UUID) bool {
	if r.redisClient != nil {
		key := failedKeyPrefix + keyID.String()
		exists, err := r.redisClient.Exists(context.Background(), key).Result()
		if err == nil {
			return exists > 0
		}
		// Redis error — fall through to in-memory
		r.logger.Debug("redis failed for key check, using in-memory fallback", zap.Error(err))
	}

	r.failedKeysMu.RLock()
	defer r.failedKeysMu.RUnlock()
	info, exists := r.failedKeys[keyID]
	if !exists {
		return false
	}
	if time.Since(info.FailedAt) > failedKeyTTL {
		return false
	}
	return true
}

// MarkKeyFailed marks an API key as temporarily failed.
// Writes to both Redis (for cross-instance) and in-memory (for fallback).
func (r *Router) MarkKeyFailed(keyID uuid.UUID, reason string) {
	// Write to Redis if available
	if r.redisClient != nil {
		key := failedKeyPrefix + keyID.String()
		if err := r.redisClient.Set(context.Background(), key, reason, failedKeyTTL).Err(); err != nil {
			r.logger.Debug("redis failed for key mark, using in-memory fallback", zap.Error(err))
		}
	}

	// Always write to in-memory as fallback
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
	if r.redisClient != nil {
		key := failedKeyPrefix + keyID.String()
		_ = r.redisClient.Del(context.Background(), key).Err()
	}
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

	return selectWeightedKey(availableKeys)
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

	return selectWeightedKey(availableKeys)
}

// selectWeightedKey selects a key from the given slice using priority-then-weighted-random.
// Keys with the lowest (best) priority value are considered first, then weighted
// random selection is applied among those keys.
func selectWeightedKey(keys []models.ProviderAPIKey) (*models.ProviderAPIKey, error) {
	if len(keys) == 0 {
		return nil, errors.New("no keys available")
	}

	// Find the best (lowest) priority among keys
	bestPriority := math.MaxInt32
	for _, k := range keys {
		prio := k.Priority
		if prio == 0 {
			prio = 1 // default priority
		}
		if prio < bestPriority {
			bestPriority = prio
		}
	}

	// Filter keys down to just the ones with the best priority
	priorityKeys := make([]models.ProviderAPIKey, 0, len(keys))
	for _, k := range keys {
		prio := k.Priority
		if prio == 0 {
			prio = 1
		}
		if prio == bestPriority {
			priorityKeys = append(priorityKeys, k)
		}
	}

	// Weighted random selection
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
