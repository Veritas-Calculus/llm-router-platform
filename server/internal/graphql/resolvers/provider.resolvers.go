package resolvers

// This file contains provider domain resolvers.
// Extracted from schema.resolvers.go for maintainability.

import (
	"context"
	"fmt"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/graphql/model"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"
	"strings"
	"time"

	"github.com/google/uuid"
)

const allowLocalProviderURLs = true

// CreateProvider is the resolver for the createProvider field.
func (r *mutationResolver) CreateProvider(ctx context.Context, input model.CreateProviderInput) (*model.Provider, error) {
	// SSRF protection: validate the URL
	if err := sanitize.ValidateWebhookURL(input.BaseURL, true, allowLocalProviderURLs); err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if existing, err := r.Router.GetProviderByName(ctx, input.Name); err == nil && existing != nil {
		return nil, fmt.Errorf("provider %q already exists", input.Name)
	}

	p := &models.Provider{
		Name:           input.Name,
		BaseURL:        input.BaseURL,
		IsActive:       false,
		Priority:       5,
		Weight:         1.0,
		MaxRetries:     3,
		Timeout:        30,
		UseProxy:       false,
		RequiresAPIKey: true,
	}

	// Apply optional overrides
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}
	if input.Priority != nil {
		p.Priority = *input.Priority
	}
	if input.Weight != nil {
		p.Weight = *input.Weight
	}
	if input.MaxRetries != nil {
		p.MaxRetries = *input.MaxRetries
	}
	if input.Timeout != nil {
		p.Timeout = *input.Timeout
	}
	if input.UseProxy != nil {
		p.UseProxy = *input.UseProxy
	}
	if input.RequiresAPIKey != nil {
		p.RequiresAPIKey = *input.RequiresAPIKey
	}

	if err := r.Router.CreateProvider(ctx, p); err != nil {
		return nil, err
	}
	return providerToGQL(p), nil
}

// DeleteProvider is the resolver for the deleteProvider field.
func (r *mutationResolver) DeleteProvider(ctx context.Context, id string) (bool, error) {
	pid, _ := uuid.Parse(id)
	if err := r.Router.DeleteProvider(ctx, pid); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateProvider is the resolver for the updateProvider field.
func (r *mutationResolver) UpdateProvider(ctx context.Context, id string, input model.ProviderInput) (*model.Provider, error) {
	pid, _ := uuid.Parse(id)
	p, err := r.Router.GetProviderByID(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("provider not found")
	}
	if input.Name != nil {
		p.Name = *input.Name
	}
	if input.BaseURL != nil {
		// Provider endpoints may be local/self-hosted, so HTTP and private IPs
		// are allowed here while still using a guarded transport at dial time.
		if err := sanitize.ValidateWebhookURL(*input.BaseURL, true, allowLocalProviderURLs); err != nil {
			return nil, fmt.Errorf("invalid base URL: %w", err)
		}
		p.BaseURL = *input.BaseURL
	}
	if input.IsActive != nil {
		p.IsActive = *input.IsActive
	}
	if input.Priority != nil {
		p.Priority = *input.Priority
	}
	if input.Weight != nil {
		p.Weight = *input.Weight
	}
	if input.MaxRetries != nil {
		p.MaxRetries = *input.MaxRetries
	}
	if input.Timeout != nil {
		p.Timeout = *input.Timeout
	}
	if input.UseProxy != nil {
		p.UseProxy = *input.UseProxy
	}
	if input.DefaultProxyID != nil {
		if *input.DefaultProxyID == "" {
			p.DefaultProxyID = nil
		} else {
			pid, _ := uuid.Parse(*input.DefaultProxyID)
			p.DefaultProxyID = &pid
		}
	}
	if input.RequiresAPIKey != nil {
		p.RequiresAPIKey = *input.RequiresAPIKey
	}
	if err := r.Router.UpdateProvider(ctx, p); err != nil {
		return nil, err
	}
	return providerToGQL(p), nil
}

// ToggleProvider is the resolver for the toggleProvider field.
func (r *mutationResolver) ToggleProvider(ctx context.Context, id string) (*model.Provider, error) {
	pid, _ := uuid.Parse(id)
	p, err := r.Router.GetProviderByID(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("provider not found")
	}
	p.IsActive = !p.IsActive
	if err := r.Router.UpdateProvider(ctx, p); err != nil {
		return nil, err
	}
	return providerToGQL(p), nil
}

// ToggleProviderProxy is the resolver for the toggleProviderProxy field.
func (r *mutationResolver) ToggleProviderProxy(ctx context.Context, id string) (*model.Provider, error) {
	pid, _ := uuid.Parse(id)
	p, err := r.Router.GetProviderByID(ctx, pid)
	if err != nil {
		return nil, fmt.Errorf("provider not found")
	}
	p.UseProxy = !p.UseProxy
	if err := r.Router.UpdateProvider(ctx, p); err != nil {
		return nil, err
	}
	return providerToGQL(p), nil
}

// CreateProviderAPIKey is the resolver for the createProviderApiKey field.
func (r *mutationResolver) CreateProviderAPIKey(ctx context.Context, providerID string, input model.ProviderAPIKeyInput) (*model.ProviderAPIKey, error) {
	pid, _ := uuid.Parse(providerID)
	input.APIKey = strings.TrimSpace(input.APIKey)
	keyPrefix := input.APIKey
	if len(keyPrefix) > 10 {
		keyPrefix = keyPrefix[:8] + "..."
	}
	encrypted, err := crypto.Encrypt(input.APIKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt API key")
	}
	prio := 1
	if input.Priority != nil {
		prio = *input.Priority
	}
	weight := 1.0
	if input.Weight != nil {
		weight = *input.Weight
	}
	rl := 0
	if input.RateLimit != nil {
		rl = *input.RateLimit
	}
	key := &models.ProviderAPIKey{
		ProviderID: pid, Alias: input.Alias, EncryptedAPIKey: encrypted,
		KeyPrefix: keyPrefix, IsActive: true, Priority: prio, Weight: weight, RateLimit: rl,
	}
	if err := r.Router.CreateProviderAPIKey(ctx, key); err != nil {
		return nil, err
	}
	return providerAPIKeyToGQL(key), nil
}

// UpdateProviderAPIKey is the resolver for the updateProviderApiKey field.
func (r *mutationResolver) UpdateProviderAPIKey(ctx context.Context, providerID string, keyID string, input model.UpdateProviderAPIKeyInput) (*model.ProviderAPIKey, error) {
	kid, _ := uuid.Parse(keyID)
	key, err := r.Router.GetProviderAPIKeyByID(ctx, kid)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}
	if input.Priority != nil {
		key.Priority = *input.Priority
	}
	if input.Weight != nil {
		key.Weight = *input.Weight
	}
	if input.RateLimit != nil {
		key.RateLimit = *input.RateLimit
	}
	if err := r.Router.UpdateProviderAPIKey(ctx, key); err != nil {
		return nil, err
	}
	return providerAPIKeyToGQL(key), nil
}

// ToggleProviderAPIKey is the resolver for the toggleProviderApiKey field.
func (r *mutationResolver) ToggleProviderAPIKey(ctx context.Context, providerID string, keyID string) (*model.ProviderAPIKey, error) {
	kid, _ := uuid.Parse(keyID)
	key, err := r.Router.ToggleProviderAPIKey(ctx, kid)
	if err != nil {
		return nil, err
	}
	return providerAPIKeyToGQL(key), nil
}

// DeleteProviderAPIKey is the resolver for the deleteProviderApiKey field.
func (r *mutationResolver) DeleteProviderAPIKey(ctx context.Context, providerID string, keyID string) (bool, error) {
	kid, _ := uuid.Parse(keyID)
	return true, r.Router.DeleteProviderAPIKey(ctx, kid)
}

// CreateModel is the resolver for the createModel field.
func (r *mutationResolver) CreateModel(ctx context.Context, providerID string, input model.ModelInput) (*model.Model, error) {
	pid, err := uuid.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("invalid provider id")
	}
	m := models.Model{
		ProviderID:  pid,
		Name:        input.Name,
		DisplayName: derefStrDefault(input.DisplayName, input.Name),
		IsActive:    derefBool(input.IsActive, true),
		MaxTokens:   valInt(input.MaxTokens, 4096),
	}
	if input.InputPricePer1k != nil {
		m.InputPricePer1K = *input.InputPricePer1k
	}
	if input.OutputPricePer1k != nil {
		m.OutputPricePer1K = *input.OutputPricePer1k
	}
	if input.PricePerSecond != nil {
		m.PricePerSecond = *input.PricePerSecond
	}
	if input.PricePerImage != nil {
		m.PricePerImage = *input.PricePerImage
	}
	if input.PricePerMinute != nil {
		m.PricePerMinute = *input.PricePerMinute
	}
	if err := r.AdminSvc.DB().Create(&m).Error; err != nil {
		return nil, fmt.Errorf("failed to create model: %w", err)
	}
	return modelToGQL(&m), nil
}

// UpdateModel is the resolver for the updateModel field.
func (r *mutationResolver) UpdateModel(ctx context.Context, id string, input model.ModelInput) (*model.Model, error) {
	mid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid model id")
	}
	var m models.Model
	if err := r.AdminSvc.DB().First(&m, "id = ?", mid).Error; err != nil {
		return nil, fmt.Errorf("model not found")
	}
	m.Name = input.Name
	if input.DisplayName != nil {
		m.DisplayName = *input.DisplayName
	}
	if input.MaxTokens != nil {
		m.MaxTokens = *input.MaxTokens
	}
	if input.IsActive != nil {
		m.IsActive = *input.IsActive
	}
	if input.InputPricePer1k != nil {
		m.InputPricePer1K = *input.InputPricePer1k
	}
	if input.OutputPricePer1k != nil {
		m.OutputPricePer1K = *input.OutputPricePer1k
	}
	if input.PricePerSecond != nil {
		m.PricePerSecond = *input.PricePerSecond
	}
	if input.PricePerImage != nil {
		m.PricePerImage = *input.PricePerImage
	}
	if input.PricePerMinute != nil {
		m.PricePerMinute = *input.PricePerMinute
	}
	if err := r.AdminSvc.DB().Save(&m).Error; err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}
	return modelToGQL(&m), nil
}

// DeleteModel is the resolver for the deleteModel field.
func (r *mutationResolver) DeleteModel(ctx context.Context, id string) (bool, error) {
	mid, err := uuid.Parse(id)
	if err != nil {
		return false, fmt.Errorf("invalid model id")
	}
	if err := r.AdminSvc.DB().Delete(&models.Model{}, "id = ?", mid).Error; err != nil {
		return false, fmt.Errorf("failed to delete model: %w", err)
	}
	return true, nil
}

// ToggleModel is the resolver for the toggleModel field.
func (r *mutationResolver) ToggleModel(ctx context.Context, id string) (*model.Model, error) {
	mid, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid model id")
	}
	var m models.Model
	if err := r.AdminSvc.DB().First(&m, "id = ?", mid).Error; err != nil {
		return nil, fmt.Errorf("model not found")
	}
	m.IsActive = !m.IsActive
	if err := r.AdminSvc.DB().Save(&m).Error; err != nil {
		return nil, fmt.Errorf("failed to toggle model: %w", err)
	}
	return modelToGQL(&m), nil
}

// SyncProviderModels is the resolver for the syncProviderModels field.
func (r *mutationResolver) SyncProviderModels(ctx context.Context, providerID string) ([]*model.Model, error) {
	pid, err := uuid.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("invalid provider id")
	}
	syncedModels, err := r.Router.SyncProviderModels(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Model, len(syncedModels))
	for i := range syncedModels {
		out[i] = modelToGQL(&syncedModels[i])
	}
	return out, nil
}

// Providers is the resolver for the providers field.
func (r *queryResolver) Providers(ctx context.Context) ([]*model.Provider, error) {
	providers, err := r.Router.GetAllProviders(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*model.Provider, len(providers))
	for i := range providers {
		out[i] = providerToGQL(&providers[i])
	}
	return out, nil
}

// ProviderAPIKeys is the resolver for the providerApiKeys field.
func (r *queryResolver) ProviderAPIKeys(ctx context.Context, providerID string) ([]*model.ProviderAPIKey, error) {
	pid, _ := uuid.Parse(providerID)
	keys, err := r.Router.GetAllProviderAPIKeys(ctx, pid)
	if err != nil {
		return nil, err
	}
	out := make([]*model.ProviderAPIKey, len(keys))
	for i := range keys {
		out[i] = providerAPIKeyToGQL(&keys[i])
	}
	return out, nil
}

// Models is the resolver for the models field.
func (r *queryResolver) Models(ctx context.Context, providerID string) ([]*model.Model, error) {
	pid, err := uuid.Parse(providerID)
	if err != nil {
		return nil, fmt.Errorf("invalid provider id")
	}
	var dbModels []models.Model
	if err := r.AdminSvc.DB().Where("provider_id = ?", pid).Order("name ASC").Find(&dbModels).Error; err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	out := make([]*model.Model, len(dbModels))
	for i := range dbModels {
		out[i] = modelToGQL(&dbModels[i])
	}
	return out, nil
}

// ProviderHealth is the resolver for the providerHealth field.
func (r *queryResolver) ProviderHealth(ctx context.Context, providerID string) (*model.ProviderHealth, error) {
	pid, _ := uuid.Parse(providerID)
	s, err := r.Health.CheckSingleProvider(ctx, pid)
	if err != nil {
		return nil, err
	}
	var lc *time.Time
	if !s.LastCheck.IsZero() {
		lc = &s.LastCheck
	}
	var em *string
	if s.ErrorMessage != "" {
		em = &s.ErrorMessage
	}
	return &model.ProviderHealth{
		ID: s.ID.String(), Name: s.Name, BaseURL: s.BaseURL,
		IsActive: s.IsActive, IsHealthy: s.IsHealthy, UseProxy: s.UseProxy,
		ResponseTime: float64(s.ResponseTime), LastCheck: lc,
		SuccessRate: s.SuccessRate, ErrorMessage: em,
	}, nil
}
