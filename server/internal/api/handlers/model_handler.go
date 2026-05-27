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
			h.fallbackModelsFromDB(ctx, p, &result)
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
		h.fallbackModelsFromDB(ctx, p, &result)
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
		h.fallbackModelsFromDB(ctx, p, &result)
		return result
	}

	if len(fetchedModels) == 0 {
		h.fallbackModelsFromDB(ctx, p, &result)
		if len(result.models) > 0 {
			return result
		}
	}

	// Reconcile upstream list with our DB. Two reasons:
	//   * Audit L-05: a model marked inactive by an admin (or by the
	//     NSFW backfill) must not leak to /v1/models even when the
	//     upstream provider still advertises it.
	//   * Audit M-07: stamp our richer context_window / max_output_tokens
	//     metadata onto the upstream payload so the playground slider
	//     uses the right cap.
	fetchedModels = h.reconcileWithDB(ctx, p, fetchedModels)

	// Cache the full model info (with extra upstream metadata)
	h.setCachedModels(p.Name, fetchedModels)
	result.models = fetchedModels
	return result
}

// reconcileWithDB filters out models whose DB row is inactive and merges
// any DB-side enrichment (model_kind, context_window, max_output_tokens)
// onto the upstream payload. Models present upstream but missing from the
// DB are kept as-is — the sync mutation is the one place that creates DB
// rows, and we don't want a side-effecting GET to spawn rows.
func (h *ModelHandler) reconcileWithDB(ctx context.Context, p models.Provider, upstream []provider.ModelInfo) []provider.ModelInfo {
	dbModels, err := h.router.GetModelsByProvider(ctx, p.ID)
	if err != nil || len(dbModels) == 0 {
		return upstream
	}

	byName := make(map[string]models.Model, len(dbModels))
	for i := range dbModels {
		byName[dbModels[i].Name] = dbModels[i]
	}

	out := make([]provider.ModelInfo, 0, len(upstream))
	for _, mi := range upstream {
		dbRow, ok := byName[mi.ID]
		if ok && !dbRow.IsActive {
			// Operator explicitly disabled this model (or it matched the
			// NSFW backfill). Drop it from /v1/models entirely.
			continue
		}
		if ok {
			if mi.Extra == nil {
				mi.Extra = map[string]json.RawMessage{}
			}
			if dbRow.ContextWindow > 0 {
				if raw, err := json.Marshal(dbRow.ContextWindow); err == nil {
					mi.Extra["context_window"] = raw
					if _, exists := mi.Extra["max_context_length"]; !exists {
						mi.Extra["max_context_length"] = raw
					}
				}
			}
			if dbRow.MaxOutputTokens != nil && *dbRow.MaxOutputTokens > 0 {
				if raw, err := json.Marshal(*dbRow.MaxOutputTokens); err == nil {
					mi.Extra["max_output_tokens"] = raw
				}
			}
			if dbRow.ModelKind != "" && dbRow.ModelKind != models.ModelKindUnknown {
				if raw, err := json.Marshal(strings.ToUpper(string(dbRow.ModelKind))); err == nil {
					mi.Extra["model_kind"] = raw
				}
			}
		}
		out = append(out, mi)
	}
	return out
}

func (h *ModelHandler) fallbackModelsFromDB(ctx context.Context, p models.Provider, result *fetchModelsResult) {
	dbModels, err := h.router.GetModelsByProvider(ctx, p.ID)
	if err != nil {
		h.logger.Debug("failed to load configured models from database",
			zap.String("provider", p.Name),
			zap.Error(err))
		return
	}

	result.models = configuredModelsToProviderInfo(dbModels)
	if len(result.models) > 0 {
		h.setCachedModels(p.Name, result.models)
	}
}

func configuredModelsToProviderInfo(dbModels []models.Model) []provider.ModelInfo {
	out := make([]provider.ModelInfo, 0, len(dbModels))
	for _, m := range dbModels {
		if !m.IsActive || m.Name == "" {
			continue
		}
		name := m.Name
		displayName := m.DisplayName
		if displayName == "" {
			displayName = name
		}

		extra := map[string]json.RawMessage{}
		if m.MaxTokens > 0 {
			if raw, err := json.Marshal(m.MaxTokens); err == nil {
				extra["max_tokens"] = raw
			}
		}
		// Audit M-07: surface context_window and max_output_tokens
		// separately so the playground slider doesn't conflate them.
		if m.ContextWindow > 0 {
			if raw, err := json.Marshal(m.ContextWindow); err == nil {
				extra["context_window"] = raw
				extra["max_context_length"] = raw
			}
		}
		if m.MaxOutputTokens != nil && *m.MaxOutputTokens > 0 {
			if raw, err := json.Marshal(*m.MaxOutputTokens); err == nil {
				extra["max_output_tokens"] = raw
			}
		}
		if m.ModelKind != "" && m.ModelKind != models.ModelKindUnknown {
			if raw, err := json.Marshal(strings.ToUpper(string(m.ModelKind))); err == nil {
				extra["model_kind"] = raw
			}
		}
		if displayName != name {
			if raw, err := json.Marshal(displayName); err == nil {
				extra["display_name"] = raw
			}
		}

		out = append(out, provider.ModelInfo{
			ID:    name,
			Name:  displayName,
			Extra: extra,
		})
	}
	return out
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
	callerKey := apiKeyFromContext(c)
	activeProviders := make([]models.Provider, 0)
	for _, p := range providers {
		if p.IsActive && (callerKey == nil || callerKey.AllowsProvider(&p)) {
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
			if callerKey != nil && !callerKey.AllowsModel(m.ID) {
				continue
			}
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
	callerKey := apiKeyFromContext(c)
	activeProviders := make([]models.Provider, 0)
	for _, p := range providers {
		if p.IsActive && (callerKey == nil || callerKey.AllowsProvider(&p)) {
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
			if callerKey != nil && !callerKey.AllowsModel(mi.ID) {
				continue
			}
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
	callerKey := apiKeyFromContext(c)
	activeProviders := make([]models.Provider, 0)
	for _, p := range allProviders {
		if p.IsActive && (callerKey == nil || callerKey.AllowsProvider(&p)) {
			activeProviders = append(activeProviders, p)
		}
	}

	if callerKey != nil && !callerKey.AllowsModel(modelID) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "The model '" + modelID + "' does not exist",
				"type":    "invalid_request_error",
				"code":    "model_not_found",
			},
		})
		return
	}

	if m, found := h.findAndFormatModel(ctx, modelID, activeProviders); found {
		c.JSON(http.StatusOK, m)
		return
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

func apiKeyFromContext(c *gin.Context) *models.APIKey {
	if c == nil {
		return nil
	}
	raw, ok := c.Get("api_key")
	if !ok {
		return nil
	}
	key, _ := raw.(*models.APIKey)
	return key
}

func (h *ModelHandler) findAndFormatModel(ctx context.Context, modelID string, activeProviders []models.Provider) (map[string]interface{}, bool) {
	for _, p := range activeProviders {
		models, ok := h.getCachedModels(p.Name)
		if !ok {
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
				for k, v := range mi.Extra {
					if k == "id" || k == "object" || k == "owned_by" {
						continue
					}
					var val json.RawMessage
					if err := json.Unmarshal(v, &val); err == nil {
						m[k] = val
					}
				}
				inferModelCapabilities(mi.ID, m)
				return m, true
			}
		}
	}
	return nil, false
}

// visionModelPatterns contains substrings that indicate a model supports vision.
var visionModelPatterns = []string{
	"-vl-", "-vl/", "/vl-", // qwen/qwen3-vl-8b, etc.
	"-vision",                        // gpt-4-vision-preview
	"vision-",                        // vision-* models
	"4o",                             // gpt-4o (multimodal)
	"gemini-pro",                     // Gemini Pro Vision
	"gemini-1.5",                     // Gemini 1.5 (multimodal)
	"gemini-2",                       // Gemini 2.x (multimodal)
	"claude-3",                       // Claude 3 (vision)
	"claude-4",                       // Claude 4 (vision)
	"pixtral",                        // Mistral Pixtral (vision)
	"llava",                          // LLaVA models
	"cogvlm",                         // CogVLM models
	"internvl",                       // InternVL models
	"minicpm-v",                      // MiniCPM-V models
	"phi-3-vision", "phi-3.5-vision", // Phi-3 Vision
	"glm-4v", "glm-4.6v", "glm-4.7v", // GLM-4V models
}

// inferModelCapabilities enriches a model's response map with capability
// metadata if the upstream provider didn't supply it. This covers providers
// like LM Studio whose /v1/models only returns {id, object, created, owned_by}.
//
// Also stamps a `model_kind` field on every response so the Playground can
// filter STT/TTS dropdowns without re-implementing the classifier
// (audit M-02). The kind is derived from the upstream payload via
// router.ClassifyModel so we have exactly one source of truth.
func inferModelCapabilities(modelID string, m map[string]interface{}) {
	// Always populate model_kind. We reconstruct a minimal ModelInfo
	// from the response map so ClassifyModel can read both the id and
	// any capability hints the upstream sent.
	if _, ok := m["model_kind"]; !ok {
		info := provider.ModelInfo{
			ID:    modelID,
			Extra: map[string]json.RawMessage{},
		}
		if raw, ok := m["capabilities"]; ok {
			if b, err := json.Marshal(raw); err == nil {
				info.Extra["capabilities"] = b
			}
		}
		if raw, ok := m["type"]; ok {
			if b, err := json.Marshal(raw); err == nil {
				info.Extra["type"] = b
			}
		}
		m["model_kind"] = strings.ToUpper(string(router.ClassifyModel(info)))
	}

	// Skip the rest if upstream already provided capabilities
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
