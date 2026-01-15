// Package repository provides database access layer.
package repository

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRepository handles user data access.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID retrieves a user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user.
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// APIKeyRepository handles API key data access.
type APIKeyRepository struct {
	db *gorm.DB
}

// NewAPIKeyRepository creates a new API key repository.
func NewAPIKeyRepository(db *gorm.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create inserts a new API key.
func (r *APIKeyRepository) Create(ctx context.Context, key *models.APIKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// GetByID retrieves an API key by ID.
func (r *APIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	var key models.APIKey
	if err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByKeyHash retrieves an API key by hash.
func (r *APIKeyRepository) GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error) {
	var key models.APIKey
	if err := r.db.WithContext(ctx).First(&key, "key_hash = ?", hash).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByUserID retrieves all API keys for a user.
func (r *APIKeyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error) {
	var keys []models.APIKey
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetActive retrieves all active API keys.
func (r *APIKeyRepository) GetActive(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// Update updates an API key.
func (r *APIKeyRepository) Update(ctx context.Context, key *models.APIKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// Delete permanently removes an API key from the database.
func (r *APIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.APIKey{}, "id = ?", id).Error
}

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

// GetActive retrieves all active providers.
func (r *ProviderRepository) GetActive(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// GetAll retrieves all providers.
func (r *ProviderRepository) GetAll(ctx context.Context) ([]models.Provider, error) {
	var providers []models.Provider
	if err := r.db.WithContext(ctx).Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// Update updates a provider.
func (r *ProviderRepository) Update(ctx context.Context, provider *models.Provider) error {
	return r.db.WithContext(ctx).Save(provider).Error
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

// Delete deletes a provider API key by ID.
func (r *ProviderAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.ProviderAPIKey{}, "id = ?", id).Error
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

// ProxyRepository handles proxy data access.
type ProxyRepository struct {
	db *gorm.DB
}

// NewProxyRepository creates a new proxy repository.
func NewProxyRepository(db *gorm.DB) *ProxyRepository {
	return &ProxyRepository{db: db}
}

// Create inserts a new proxy.
func (r *ProxyRepository) Create(ctx context.Context, proxy *models.Proxy) error {
	return r.db.WithContext(ctx).Create(proxy).Error
}

// GetByID retrieves a proxy by ID.
func (r *ProxyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Proxy, error) {
	var proxy models.Proxy
	if err := r.db.WithContext(ctx).First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

// GetActive retrieves all active proxies.
func (r *ProxyRepository) GetActive(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// GetAll retrieves all proxies.
func (r *ProxyRepository) GetAll(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// Update updates a proxy.
func (r *ProxyRepository) Update(ctx context.Context, proxy *models.Proxy) error {
	return r.db.WithContext(ctx).Save(proxy).Error
}

// Delete deletes a proxy.
func (r *ProxyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Proxy{}, "id = ?", id).Error
}

// UsageLogRepository handles usage log data access.
type UsageLogRepository struct {
	db *gorm.DB
}

// NewUsageLogRepository creates a new usage log repository.
func NewUsageLogRepository(db *gorm.DB) *UsageLogRepository {
	return &UsageLogRepository{db: db}
}

// Create inserts a new usage log.
func (r *UsageLogRepository) Create(ctx context.Context, log *models.UsageLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByUserIDAndTimeRange retrieves usage logs for a user in time range.
func (r *UsageLogRepository) GetByUserIDAndTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, start, end).
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// HealthHistoryRepository handles health history data access.
type HealthHistoryRepository struct {
	db *gorm.DB
}

// NewHealthHistoryRepository creates a new health history repository.
func NewHealthHistoryRepository(db *gorm.DB) *HealthHistoryRepository {
	return &HealthHistoryRepository{db: db}
}

// Create inserts a new health history record.
func (r *HealthHistoryRepository) Create(ctx context.Context, history *models.HealthHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

// GetByTarget retrieves health history for a target.
func (r *HealthHistoryRepository) GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID, limit int) ([]models.HealthHistory, error) {
	var histories []models.HealthHistory
	if err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("checked_at DESC").
		Limit(limit).
		Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// GetRecent retrieves recent health history.
func (r *HealthHistoryRepository) GetRecent(ctx context.Context, targetType string, limit int) ([]models.HealthHistory, error) {
	var histories []models.HealthHistory
	query := r.db.WithContext(ctx).Order("checked_at DESC").Limit(limit)
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if err := query.Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// ConversationMemoryRepository handles conversation memory data access.
type ConversationMemoryRepository struct {
	db *gorm.DB
}

// NewConversationMemoryRepository creates a new conversation memory repository.
func NewConversationMemoryRepository(db *gorm.DB) *ConversationMemoryRepository {
	return &ConversationMemoryRepository{db: db}
}

// Create inserts a new conversation memory.
func (r *ConversationMemoryRepository) Create(ctx context.Context, memory *models.ConversationMemory) error {
	return r.db.WithContext(ctx).Create(memory).Error
}

// GetByConversation retrieves messages for a conversation.
func (r *ConversationMemoryRepository) GetByConversation(ctx context.Context, userID uuid.UUID, conversationID string) ([]models.ConversationMemory, error) {
	var memories []models.ConversationMemory
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("sequence ASC").
		Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

// DeleteByConversation deletes all messages in a conversation.
func (r *ConversationMemoryRepository) DeleteByConversation(ctx context.Context, userID uuid.UUID, conversationID string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Delete(&models.ConversationMemory{}).Error
}

// ListConversationIDs returns all conversation IDs for a user.
func (r *ConversationMemoryRepository) ListConversationIDs(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMemory{}).
		Where("user_id = ?", userID).
		Distinct("conversation_id").
		Pluck("conversation_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
