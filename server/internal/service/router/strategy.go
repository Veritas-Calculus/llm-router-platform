// Package router provides LLM request routing logic.
// This file contains routing strategy implementations.
package router

import (
	"cmp"
	"context"
	"math"
	"path"
	"slices"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// heuristicPrefixes maps provider name to model name prefixes for last-resort heuristic matching.
var heuristicPrefixes = map[string][]string{
	"google":    {"gemini", "gemma", "embedding", "text-embedding", "imagen", "veo", "aqa"},
	"openai":    {"gpt-", "o1", "o3", "o4", "chatgpt", "text-davinci", "dall-e", "whisper", "tts"},
	"anthropic": {"claude"},
	"deepseek":  {"deepseek"},
	"mistral":   {"mistral", "mixtral", "codestral", "pixtral", "open-mistral", "open-mixtral"},
}

// heuristicContains maps provider name to model name substrings for local/self-hosted providers.
var heuristicContains = map[string][]string{
	"ollama":   {"llama", "codellama", "vicuna", "phi", "yi-", "qwen", "mistral", "cosyvoice", "fish-speech", "chattts", "bark"},
	"lmstudio": {"llama", "codellama", "vicuna", "phi", "yi-", "qwen", "mistral", "cosyvoice", "fish-speech", "chattts", "bark"},
	"vllm":     {"llama", "codellama", "vicuna", "phi", "yi-", "qwen", "mistral", "cosyvoice", "fish-speech", "chattts", "bark"},
}

// findProviderForModel tries to find the appropriate provider for a given model name.
// It strips client-format prefixes (e.g., "openai/gpt-oss-120b" -> "gpt-oss-120b"),
// then prioritises explicit DB model assignments over heuristic prefix matching.
func (r *Router) findProviderForModel(modelName string, providers []models.Provider) *models.Provider {
	// Strip client prefix if present (e.g., "openai/gpt-oss-120b" -> "gpt-oss-120b").
	actualModel := modelName
	if idx := strings.Index(modelName, "/"); idx > 0 {
		actualModel = modelName[idx+1:]
	}

	// 1. Check cached DB model assignments (refreshed every 5 minutes).
	if r.modelRepo != nil {
		modelMap := r.getModelProviderCache(providers)
		if providerIdx, ok := modelMap[strings.ToLower(actualModel)]; ok {
			r.logger.Debug("model matched via database cache",
				zap.String("model", sanitize.LogValue(modelName)),
				zap.String("provider", providers[providerIdx].Name),
			)
			return &providers[providerIdx]
		}
	}

	// 2. Cached upstream model discovery.
	discoveryMap := r.getDiscoveryCache()
	if discoveryMap == nil {
		discoveryMap = r.refreshDiscoveryCache(providers)
	}
	if providerName, ok := discoveryMap[strings.ToLower(actualModel)]; ok {
		for i := range providers {
			if strings.EqualFold(providers[i].Name, providerName) {
				r.logger.Debug("model matched via upstream discovery cache",
					zap.String("model", sanitize.LogValue(modelName)),
					zap.String("provider", providerName),
				)
				return &providers[i]
			}
		}
	}

	modelLower := strings.ToLower(actualModel)

	// 3. Configurable model patterns from Provider.ModelPatterns.
	for i := range providers {
		patterns := providers[i].GetModelPatterns()
		if len(patterns) == 0 {
			continue
		}
		for _, pattern := range patterns {
			if matchesGlobPattern(modelLower, strings.ToLower(pattern)) {
				r.logger.Debug("model matched via configured patterns",
					zap.String("model", sanitize.LogValue(modelName)),
					zap.String("provider", providers[i].Name),
					zap.String("pattern", pattern),
				)
				return &providers[i]
			}
		}
	}

	// 4. Heuristic fallback (data-driven).
	if p := r.matchHeuristicFallback(modelLower, providers); p != nil {
		return p
	}

	return nil
}

// matchHeuristicFallback uses data-driven maps to match a model name to a provider
// via prefix or substring matching. This replaces the former switch-case block.
func (r *Router) matchHeuristicFallback(modelLower string, providers []models.Provider) *models.Provider {
	for i := range providers {
		p := &providers[i]
		// Check prefix-based matching
		if prefixes, ok := heuristicPrefixes[p.Name]; ok {
			for _, prefix := range prefixes {
				if strings.HasPrefix(modelLower, prefix) {
					return p
				}
			}
		}
		// Check substring-based matching (local providers)
		if substrings, ok := heuristicContains[p.Name]; ok {
			for _, substr := range substrings {
				if strings.Contains(modelLower, substr) {
					return p
				}
			}
		}
	}
	return nil
}

// matchesGlobPattern checks if a model name matches a glob-style pattern.
// Supports "*" (match any sequence) and "?" (match single char) wildcards.
// Pattern matching is case-insensitive (both inputs should be lowercased).
func matchesGlobPattern(modelName, pattern string) bool {
	// Use path.Match for glob-style matching.
	matched, err := path.Match(pattern, modelName)
	if err != nil {
		return false
	}
	return matched
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

// selectLeastLatency selects the provider with the lowest observed latency.
// Uses EWMA (exponentially weighted moving average) data from RecordLatency().
// Falls back to weighted selection when no latency data exists.
func (r *Router) selectLeastLatency(providers []models.Provider) *models.Provider {
	r.latencyMu.RLock()
	defer r.latencyMu.RUnlock()

	var bestProvider *models.Provider
	bestLatency := int64(math.MaxInt64)
	hasData := false

	for i := range providers {
		if avg, ok := r.providerLatency[providers[i].ID]; ok && avg > 0 {
			hasData = true
			if avg < bestLatency {
				bestLatency = avg
				bestProvider = &providers[i]
			}
		}
	}

	if !hasData || bestProvider == nil {
		return r.selectWeighted(providers)
	}

	return bestProvider
}

// RecordLatency records the observed latency for a provider.
// Uses EWMA with α=0.3 to smooth out spikes while staying responsive.
func (r *Router) RecordLatency(providerID uuid.UUID, latencyMs int64) {
	r.latencyMu.Lock()
	defer r.latencyMu.Unlock()

	if r.providerLatency == nil {
		r.providerLatency = make(map[uuid.UUID]int64)
	}

	current, exists := r.providerLatency[providerID]
	if !exists {
		r.providerLatency[providerID] = latencyMs
		return
	}

	// EWMA: new = α * sample + (1-α) * old, with α = 0.3
	const alpha = 0.3
	r.providerLatency[providerID] = int64(alpha*float64(latencyMs) + (1-alpha)*float64(current))
}

// selectCostOptimized selects the provider with the lowest cost for a given model.
// It compares input_price_per_1k across all providers that offer the requested model.
// If cost data is unavailable, it falls back to weighted selection.
func (r *Router) selectCostOptimized(ctx context.Context, modelName string, providers []models.Provider) *models.Provider {
	type providerCost struct {
		provider *models.Provider
		cost     float64
	}

	var candidates []providerCost

	for i := range providers {
		p := &providers[i]
		models, err := r.modelRepo.GetByProvider(ctx, p.ID)
		if err != nil {
			continue
		}
		for _, m := range models {
			if strings.EqualFold(m.Name, modelName) && m.IsActive {
				candidates = append(candidates, providerCost{
					provider: p,
					cost:     m.InputPricePer1K,
				})
				break
			}
		}
	}

	if len(candidates) == 0 {
		// No cost data — fallback to weighted
		return r.selectWeighted(providers)
	}

	// Find the lowest cost
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.cost < best.cost {
			best = c
		}
	}

	r.logger.Debug("cost-optimized routing",
		zap.String("model", sanitize.LogValue(modelName)),
		zap.String("provider", best.provider.Name),
		zap.Float64("cost_per_1k", best.cost),
	)

	return best.provider
}

// sortByPriority sorts providers by priority descending.
func sortByPriority(providers []models.Provider) {
	slices.SortFunc(providers, func(a, b models.Provider) int {
		return cmp.Compare(b.Priority, a.Priority) // descending
	})
}
