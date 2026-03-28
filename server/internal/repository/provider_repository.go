// Package repository provides database access layer.
// This file contains provider and model data access operations.
package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProviderRepository handles provider data access.
type ProviderRepository struct {
	db *gorm.DB
}

// NewProviderRepository creates a new provider repository.
func NewProviderRepository(db *gorm.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

// Create inserts a new provider.
func (r *ProviderRepository) Create(ctx context.Context, provider *models.Provider) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// GetByID retrieves a provider by ID.
func (r *ProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.WithContext(ctx).First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByName retrieves a provider by name.
func (r *ProviderRepository) GetByName(ctx context.Context, name string) (*models.Provider, error) {
	var provider models.Provider
	if err := r.db.WithContext(ctx).First(&provider, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetActive retrieves all active providers with their models.
func (r *ProviderRepository) GetActive(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	if err := r.db.WithContext(ctx).Preload("Models").Where("is_active = ?", true).Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// GetAll retrieves all providers with their models.
func (r *ProviderRepository) GetAll(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	if err := r.db.WithContext(ctx).Preload("Models").Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// Update updates a provider.
func (r *ProviderRepository) Update(ctx context.Context, provider *models.Provider) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// Delete permanently removes a provider by ID.
func (r *ProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.Provider{}, "id = ?", id).Error
}

// ProviderAPIKeyRepository handles provider API key data access.
type ProviderAPIKeyRepository struct {
	db *gorm.DB
}

// NewProviderAPIKeyRepository creates a new provider API key repository.
func NewProviderAPIKeyRepository(db *gorm.DB) *ProviderAPIKeyRepository {
	return &ProviderAPIKeyRepository{db: db}
}

// Create inserts a new provider API key.
func (r *ProviderAPIKeyRepository) Create(ctx context.Context, key *models.ProviderAPIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// GetActiveByProvider retrieves active API keys for a provider.
func (r *ProviderAPIKeyRepository) GetActiveByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	var keys []models.ProviderAPIKey
	if err := r.db.WithContext(ctx).Where("provider_id = ? AND is_active = ?", providerID, true).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetByProvider retrieves all API keys for a provider (including inactive).
func (r *ProviderAPIKeyRepository) GetByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error) {
	var keys []models.ProviderAPIKey
	if err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetByID retrieves a provider API key by ID.
func (r *ProviderAPIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ProviderAPIKey, error) {
	var key models.ProviderAPIKey
	if err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// GetAll retrieves all provider API keys.
func (r *ProviderAPIKeyRepository) GetAll(ctx context.Context) ([]models.ProviderAPIKey, error) {
	var keys []models.ProviderAPIKey
	if err := r.db.WithContext(ctx).Preload("Provider").Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// Update updates a provider API key.
func (r *ProviderAPIKeyRepository) Update(ctx context.Context, key *models.ProviderAPIKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// Delete permanently removes a provider API key by ID.
func (r *ProviderAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.ProviderAPIKey{}, "id = ?", id).Error
}

// ModelRepository handles model data access.
type ModelRepository struct {
	db *gorm.DB
}

// NewModelRepository creates a new model repository.
func NewModelRepository(db *gorm.DB) *ModelRepository {
	return &ModelRepository{db: db}
}

// GetByID retrieves a model by ID.
func (r *ModelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	var model models.Model
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByName retrieves a model by name.
func (r *ModelRepository) GetByName(ctx context.Context, name string) (*models.Model, error) {
	var model models.Model
	if err := r.db.WithContext(ctx).First(&model, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

// GetByProvider retrieves all models for a provider.
func (r *ModelRepository) GetByProvider(ctx context.Context, providerID uuid.UUID) ([]models.Model, error) {
	var modelsList []models.Model
	if err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Find(&modelsList).Error; err != nil {
		return nil, err
	}
	return modelsList, nil
}

// GetByProviderSorted retrieves all models for a provider ordered by name.
func (r *ModelRepository) GetByProviderSorted(ctx context.Context, providerID uuid.UUID) ([]models.Model, error) {
	var modelsList []models.Model
	if err := r.db.WithContext(ctx).Where("provider_id = ?", providerID).Order("name ASC").Find(&modelsList).Error; err != nil {
		return nil, err
	}
	return modelsList, nil
}

// Create inserts a new model.
func (r *ModelRepository) Create(ctx context.Context, m *models.Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// Update saves a model.
func (r *ModelRepository) Update(ctx context.Context, m *models.Model) error {
	return r.db.WithContext(ctx).Save(m).Error
}

// Delete removes a model by ID.
func (r *ModelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Model{}, "id = ?", id).Error
}

