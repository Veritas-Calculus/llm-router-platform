// Package handlers provides HTTP request handlers.
// This file implements model listing endpoints.
package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/provider"
	"llm-router-platform/internal/service/router"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ModelHandler handles model listing endpoints.
type ModelHandler struct {
	router      *router.Router
	registry    *provider.Registry
	logger      *zap.Logger
	modelCache  map[string]*modelCacheEntry
	cacheMutex  sync.RWMutex
	cacheExpiry time.Duration
}

// modelCacheEntry holds cached model data for a provider.
type modelCacheEntry struct {
	models    []string
	fetchedAt time.Time
}

// NewModelHandler creates a new model handler.
func NewModelHandler(r *router.Router, registry *provider.Registry, logger *zap.Logger) *ModelHandler {
	return &ModelHandler{
		router:      r,
		registry:    registry,
		logger:      logger,
		modelCache:  make(map[string]*modelCacheEntry),
		cacheExpiry: 5 * time.Minute, // Cache models for 5 minutes
	}
}

// ProviderInfo represents provider information for API response.
type ProviderInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	BaseURL  string   `json:"base_url"`
	IsActive bool     `json:"is_active"`
	Models   []string `json:"models"`
}

// fetchModelsResult holds the result of fetching models for a provider.
type fetchModelsResult struct {
	providerID   string
	providerName string
	baseURL      string
	isActive     bool
	models       []string
	err          error
}

// getCachedModels returns cached models for a provider if available and not expired.
func (h *ModelHandler) getCachedModels(providerName string) ([]string, bool) {
	h.cacheMutex.RLock()
	defer h.cacheMutex.RUnlock()

	entry, ok := h.modelCache[providerName]
	if !ok {
		return nil, false
	}

	if time.Since(entry.fetchedAt) > h.cacheExpiry {
		return nil, false
	}

	return entry.models, true
}

// setCachedModels stores models in cache for a provider.
func (h *ModelHandler) setCachedModels(providerName string, models []string) {
	h.cacheMutex.Lock()
	defer h.cacheMutex.Unlock()

	h.modelCache[providerName] = &modelCacheEntry{
		models:    models,
		fetchedAt: time.Now(),
	}
}

// fetchModelsForProvider fetches models for a single provider.
func (h *ModelHandler) fetchModelsForProvider(ctx context.Context, p models.Provider) fetchModelsResult {
	result := fetchModelsResult{
		providerID:   p.ID.String(),
		providerName: p.Name,
		baseURL:      p.BaseURL,
		isActive:     p.IsActive,
		models:       []string{},
	}

	// Check cache first
	if cachedModels, ok := h.getCachedModels(p.Name); ok {
		result.models = cachedModels
		return result
	}

	// Get a client for this provider
	var client provider.Client
	var clientErr error

	if p.RequiresAPIKey {
		keys, err := h.router.GetProviderAPIKeys(ctx, p.ID)
		if err != nil || len(keys) == 0 {
			h.logger.Debug("no API key available for provider", zap.String("provider", p.Name))
			return result
		}
		client, clientErr = h.router.GetProviderClientWithKey(ctx, &p, &keys[0])
	} else {
		client, clientErr = h.router.GetProviderClientWithKey(ctx, &p, nil)
	}

	if clientErr != nil {
		h.logger.Debug("failed to create client for provider",
			zap.String("provider", p.Name),
			zap.Error(clientErr))
		result.err = clientErr
		return result
	}

	// Create a timeout context for fetching models (3 seconds max per provider)
	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Fetch models from upstream provider
	fetchedModels, err := client.ListModels(fetchCtx)
	if err != nil {
		h.logger.Debug("failed to fetch models from provider",
			zap.String("provider", p.Name),
			zap.Error(err))
		result.err = err
		return result
	}

	modelNames := make([]string, 0, len(fetchedModels))
	for _, m := range fetchedModels {
		modelNames = append(modelNames, m.ID)
	}

	// Cache the result
	h.setCachedModels(p.Name, modelNames)
	result.models = modelNames
	return result
}

// ListProviders returns available providers with their models.
func (h *ModelHandler) ListProviders(c *gin.Context) {
	ctx := c.Request.Context()
	providers, err := h.router.GetAllProviders(ctx)
	if err != nil {
		h.logger.Error("failed to list providers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list providers"})
		return
	}

	// Filter active providers
	activeProviders := make([]models.Provider, 0)
	for _, p := range providers {
		if p.IsActive {
			activeProviders = append(activeProviders, p)
		}
	}

	// Fetch models concurrently for all providers
	resultChan := make(chan fetchModelsResult, len(activeProviders))
	var wg sync.WaitGroup

	for _, p := range activeProviders {
		wg.Add(1)
		go func(prov models.Provider) {
			defer wg.Done()
			resultChan <- h.fetchModelsForProvider(ctx, prov)
		}(p)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	result := make([]ProviderInfo, 0, len(activeProviders))
	for r := range resultChan {
		result = append(result, ProviderInfo{
			ID:       r.providerID,
			Name:     r.providerName,
			BaseURL:  r.baseURL,
			IsActive: r.isActive,
			Models:   r.models,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// List returns available models.
func (h *ModelHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	// OpenAI standard model format
	type Model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	// Get all active providers
	providers, err := h.router.GetAllProviders(ctx)
	if err != nil {
		h.logger.Error("failed to get providers", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get providers"})
		return
	}

	// Filter active providers
	activeProviders := make([]models.Provider, 0)
	for _, p := range providers {
		if p.IsActive {
			activeProviders = append(activeProviders, p)
		}
	}

	// Fetch models concurrently for all providers
	resultChan := make(chan fetchModelsResult, len(activeProviders))
	var wg sync.WaitGroup

	for _, p := range activeProviders {
		wg.Add(1)
		go func(prov models.Provider) {
			defer wg.Done()
			resultChan <- h.fetchModelsForProvider(ctx, prov)
		}(p)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results in OpenAI format
	now := time.Now().Unix()
	allModels := make([]Model, 0)
	for r := range resultChan {
		for _, modelID := range r.models {
			allModels = append(allModels, Model{
				ID:      modelID,
				Object:  "model",
				Created: now,
				OwnedBy: r.providerName,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   allModels,
	})
}
