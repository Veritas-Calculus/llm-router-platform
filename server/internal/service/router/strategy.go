// Package router provides LLM request routing logic.
// This file contains routing strategy implementations.
package router

import (
	"context"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
)

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
		case "ollama", "lmstudio", "vllm":
			// Check for common open-source model patterns
			if strings.Contains(modelLower, "llama") ||
				strings.Contains(modelLower, "codellama") ||
				strings.Contains(modelLower, "vicuna") ||
				strings.Contains(modelLower, "phi") ||
				strings.Contains(modelLower, "yi-") {
				return p
			}
		case "deepseek":
			// DeepSeek models
			if strings.HasPrefix(modelLower, "deepseek") {
				return p
			}
		case "mistral":
			// Mistral AI models
			if strings.HasPrefix(modelLower, "mistral") ||
				strings.HasPrefix(modelLower, "mixtral") ||
				strings.HasPrefix(modelLower, "codestral") ||
				strings.HasPrefix(modelLower, "pixtral") ||
				strings.HasPrefix(modelLower, "open-mistral") ||
				strings.HasPrefix(modelLower, "open-mixtral") {
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
	for i := 0; i < len(providers)-1; i++ {
		for j := i + 1; j < len(providers); j++ {
			if providers[j].Priority > providers[i].Priority {
				providers[i], providers[j] = providers[j], providers[i]
			}
		}
	}
}
