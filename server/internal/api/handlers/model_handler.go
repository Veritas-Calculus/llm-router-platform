// Package handlers provides HTTP request handlers.
// This file implements model listing endpoints.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
	models    []provider.ModelInfo
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
	models       []provider.ModelInfo
	err          error
}

// getCachedModels returns cached models for a provider if available and not expired.
func (h *ModelHandler) getCachedModels(providerName string) ([]provider.ModelInfo, bool) {
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
func (h *ModelHandler) setCachedModels(providerName string, mdls []provider.ModelInfo) {
	h.cacheMutex.Lock()
	defer h.cacheMutex.Unlock()

	h.modelCache[providerName] = &modelCacheEntry{
		models:    mdls,
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
		models:       []provider.ModelInfo{},
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

	// Cache the full model info (with extra upstream metadata)
	h.setCachedModels(p.Name, fetchedModels)
	result.models = fetchedModels
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
		modelNames := make([]string, 0, len(r.models))
		for _, m := range r.models {
			modelNames = append(modelNames, m.ID)
		}
		result = append(result, ProviderInfo{
			ID:       r.providerID,
			Name:     r.providerName,
			BaseURL:  r.baseURL,
			IsActive: r.isActive,
			Models:   modelNames,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": result})
}

// List returns available models in OpenAI-compatible format.
// Extra upstream fields (e.g., type, capabilities, input_modalities) are
// forwarded transparently so clients can detect vision/multimodal support.
func (h *ModelHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

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

	// Collect results in OpenAI format, preserving extra upstream fields
	now := time.Now().Unix()
	allModels := make([]map[string]interface{}, 0)
	for r := range resultChan {
		for _, mi := range r.models {
			m := map[string]interface{}{
				"id":       mi.ID,
				"object":   "model",
				"created":  mi.Created,
				"owned_by": r.providerName,
			}
			if mi.Created == 0 {
				m["created"] = now
			}
			// Forward all extra upstream fields (type, capabilities,
			// input_modalities, output_modalities, etc.)
			for k, v := range mi.Extra {
				// Don't overwrite our standard fields
				if k == "id" || k == "object" || k == "owned_by" {
					continue
				}
				var val json.RawMessage
				if err := json.Unmarshal(v, &val); err == nil {
					m[k] = val
				}
			}

			// Infer capabilities from model name if upstream didn't
			// provide them. This is essential for local providers like
			// LM Studio that don't include capability metadata in
			// their /v1/models responses.
			inferModelCapabilities(mi.ID, m)

			allModels = append(allModels, m)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   allModels,
	})
}

// Retrieve returns details for a specific model by ID.
// Implements the standard OpenAI API: GET /v1/models/{model_id}
// Route pattern: /models/:org/*name handles IDs like "qwen/qwen3-vl-8b"
// where :org = "qwen" and *name = "/qwen3-vl-8b".
func (h *ModelHandler) Retrieve(c *gin.Context) {
	// Construct model ID from route params
	org := c.Param("org")
	name := strings.TrimPrefix(c.Param("name"), "/")

	var modelID string
	if name == "" {
		modelID = org // Simple ID like "gpt-4"
	} else {
		modelID = org + "/" + name // Slashed ID like "qwen/qwen3-vl-8b"
	}

	if modelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "model ID is required",
				"type":    "invalid_request_error",
			},
		})
		return
	}

	// Search for the model across all providers
	ctx := c.Request.Context()
	allProviders, err := h.router.GetAllProviders(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "failed to get providers",
				"type":    "server_error",
			},
		})
		return
	}

	// Filter active providers
	activeProviders := make([]models.Provider, 0)
	for _, p := range allProviders {
		if p.IsActive {
			activeProviders = append(activeProviders, p)
		}
	}

	// Try cache first, then fetch if not cached
	for _, p := range activeProviders {
		models, ok := h.getCachedModels(p.Name)
		if !ok {
			// Fetch models for this provider to populate cache
			result := h.fetchModelsForProvider(ctx, p)
			models = result.models
		}

		for _, mi := range models {
			if mi.ID == modelID {
				now := time.Now().Unix()
				m := map[string]interface{}{
					"id":       mi.ID,
					"object":   "model",
					"created":  mi.Created,
					"owned_by": p.Name,
				}
				if mi.Created == 0 {
					m["created"] = now
				}
				// Forward extra upstream fields
				for k, v := range mi.Extra {
					if k == "id" || k == "object" || k == "owned_by" {
						continue
					}
					var val json.RawMessage
					if err := json.Unmarshal(v, &val); err == nil {
						m[k] = val
					}
				}
				// Infer capabilities from model name
				inferModelCapabilities(mi.ID, m)

				c.JSON(http.StatusOK, m)
				return
			}
		}
	}

	// Model not found
	c.JSON(http.StatusNotFound, gin.H{
		"error": gin.H{
			"message": "The model '" + modelID + "' does not exist",
			"type":    "invalid_request_error",
			"code":    "model_not_found",
		},
	})
}

// visionModelPatterns contains substrings that indicate a model supports vision.
var visionModelPatterns = []string{
	"-vl-", "-vl/", "/vl-",           // qwen/qwen3-vl-8b, etc.
	"-vision",                          // gpt-4-vision-preview
	"vision-",                          // vision-* models
	"4o",                               // gpt-4o (multimodal)
	"gemini-pro",                       // Gemini Pro Vision
	"gemini-1.5",                       // Gemini 1.5 (multimodal)
	"gemini-2",                         // Gemini 2.x (multimodal)
	"claude-3",                         // Claude 3 (vision)
	"claude-4",                         // Claude 4 (vision)
	"pixtral",                          // Mistral Pixtral (vision)
	"llava",                            // LLaVA models
	"cogvlm",                           // CogVLM models
	"internvl",                         // InternVL models
	"minicpm-v",                        // MiniCPM-V models
	"phi-3-vision", "phi-3.5-vision",   // Phi-3 Vision
	"glm-4v", "glm-4.6v", "glm-4.7v",  // GLM-4V models
}

// inferModelCapabilities enriches a model's response map with capability
// metadata if the upstream provider didn't supply it. This covers providers
// like LM Studio whose /v1/models only returns {id, object, created, owned_by}.
func inferModelCapabilities(modelID string, m map[string]interface{}) {
	// Skip if upstream already provided capabilities
	if _, ok := m["capabilities"]; ok {
		return
	}
	if _, ok := m["input_modalities"]; ok {
		return
	}

	lower := strings.ToLower(modelID)
	isVision := false
	for _, pattern := range visionModelPatterns {
		if strings.Contains(lower, pattern) {
			isVision = true
			break
		}
	}

	if isVision {
		m["capabilities"] = map[string]bool{
			"vision":     true,
			"chat":       true,
			"completion": true,
		}
		m["input_modalities"] = []string{"text", "image"}
		m["output_modalities"] = []string{"text"}
		m["type"] = "vlm"
	} else {
		m["capabilities"] = map[string]bool{
			"chat":       true,
			"completion": true,
		}
		m["input_modalities"] = []string{"text"}
		m["output_modalities"] = []string{"text"}
		m["type"] = "llm"
	}
}

