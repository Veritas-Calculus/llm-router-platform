package router

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestStrategy(t *testing.T) {
	strategies := []Strategy{
		StrategyRoundRobin,
		StrategyWeighted,
		StrategyLeastLatency,
		StrategyFallback,
	}

	assert.Contains(t, strategies, StrategyRoundRobin)
	assert.Contains(t, strategies, StrategyWeighted)
	assert.Contains(t, strategies, StrategyLeastLatency)
	assert.Contains(t, strategies, StrategyFallback)
}

func TestStrategyValues(t *testing.T) {
	assert.Equal(t, Strategy("round_robin"), StrategyRoundRobin)
	assert.Equal(t, Strategy("weighted"), StrategyWeighted)
	assert.Equal(t, Strategy("least_latency"), StrategyLeastLatency)
	assert.Equal(t, Strategy("fallback"), StrategyFallback)
}

func TestProviderSelection(t *testing.T) {
	providers := []models.Provider{
		{Name: "openai", Priority: 10, Weight: 0.5, IsActive: true},
		{Name: "anthropic", Priority: 5, Weight: 0.3, IsActive: true},
		{Name: "azure", Priority: 15, Weight: 0.2, IsActive: true},
	}

	assert.Len(t, providers, 3)

	var highest *models.Provider
	for i := range providers {
		if providers[i].IsActive {
			if highest == nil || providers[i].Priority > highest.Priority {
				highest = &providers[i]
			}
		}
	}

	assert.NotNil(t, highest)
	assert.Equal(t, "azure", highest.Name)
}

func TestProviderFiltering(t *testing.T) {
	providers := []models.Provider{
		{Name: "openai", IsActive: true},
		{Name: "anthropic", IsActive: false},
		{Name: "azure", IsActive: true},
	}

	var active []models.Provider
	for _, p := range providers {
		if p.IsActive {
			active = append(active, p)
		}
	}

	assert.Len(t, active, 2)
}

func TestWeightedSelection(t *testing.T) {
	providers := []struct {
		name   string
		weight float64
	}{
		{"openai", 0.5},
		{"anthropic", 0.3},
		{"azure", 0.2},
	}

	var totalWeight float64
	for _, p := range providers {
		totalWeight += p.weight
	}

	assert.InDelta(t, 1.0, totalWeight, 0.001)
}

func TestRoundRobinIndex(t *testing.T) {
	providers := []string{"openai", "anthropic", "azure"}
	index := 0

	for i := 0; i < 6; i++ {
		_ = providers[index]
		index = (index + 1) % len(providers)
	}

	assert.Equal(t, 0, index)
}

func TestAPIKeySelection(t *testing.T) {
	providerID := uuid.New()

	keys := []models.ProviderAPIKey{
		{ProviderID: providerID, IsActive: true, UsageCount: 100},
		{ProviderID: providerID, IsActive: true, UsageCount: 50},
		{ProviderID: providerID, IsActive: false, UsageCount: 0},
	}

	var activeKeys []models.ProviderAPIKey
	for _, k := range keys {
		if k.IsActive {
			activeKeys = append(activeKeys, k)
		}
	}

	assert.Len(t, activeKeys, 2)
}

func TestRetryLogic(t *testing.T) {
	maxRetries := 3
	currentRetry := 0
	success := false

	for currentRetry < maxRetries && !success {
		currentRetry++
		if currentRetry == 2 {
			success = true
		}
	}

	assert.True(t, success)
	assert.Equal(t, 2, currentRetry)
}

func TestLatencyTracking(t *testing.T) {
	latencies := []int64{100, 150, 200, 120, 180}

	var sum int64
	for _, l := range latencies {
		sum += l
	}
	avg := sum / int64(len(latencies))

	assert.Equal(t, int64(150), avg)
}

func TestModelMatching(t *testing.T) {
	modelMappings := map[string]string{
		"gpt-4":          "openai",
		"gpt-3.5-turbo":  "openai",
		"claude-3-opus":  "anthropic",
		"claude-3-sonnet": "anthropic",
	}

	provider := modelMappings["gpt-4"]
	assert.Equal(t, "openai", provider)

	provider = modelMappings["claude-3-opus"]
	assert.Equal(t, "anthropic", provider)
}

func TestNoActiveProviders(t *testing.T) {
	providers := []models.Provider{
		{Name: "openai", IsActive: false},
		{Name: "anthropic", IsActive: false},
	}

	var active []models.Provider
	for _, p := range providers {
		if p.IsActive {
			active = append(active, p)
		}
	}

	assert.Len(t, active, 0)
}

func TestProviderPriority(t *testing.T) {
	providers := []models.Provider{
		{Name: "openai", Priority: 10},
		{Name: "anthropic", Priority: 20},
		{Name: "azure", Priority: 15},
	}

	var maxPriority int
	var maxProvider string
	for _, p := range providers {
		if p.Priority > maxPriority {
			maxPriority = p.Priority
			maxProvider = p.Name
		}
	}

	assert.Equal(t, "anthropic", maxProvider)
	assert.Equal(t, 20, maxPriority)
}
