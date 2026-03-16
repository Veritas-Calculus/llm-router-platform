// Package repository provides database access layer.
// This file contains user-related data access operations.
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

// GetAll retrieves all users (for admin).
func (r *UserRepository) GetAll(ctx context.Context) ([]models.User, error) {
	var users []models.User
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// Count returns total user count.
func (r *UserRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountActiveUsers counts users who have usage logs since a given time.
func (r *UserRepository) CountActiveUsers(ctx context.Context, since time.Time) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&models.UsageLog{}).
		Where("created_at >= ?", since).
		Distinct("user_id").
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Search finds users matching a query string (email or name).
func (r *UserRepository) Search(ctx context.Context, query string) ([]models.User, error) {
	var users []models.User
	pattern := "%" + query + "%"
	if err := r.db.WithContext(ctx).
		Where("email ILIKE ? OR name ILIKE ?", pattern, pattern).
		Order("created_at DESC").
		Limit(100).
		Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
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

// GetAll retrieves all API keys (for admin view).
func (r *APIKeyRepository) GetAll(ctx context.Context) ([]models.APIKey, error) {
	var keys []models.APIKey
	if err := r.db.WithContext(ctx).Find(&keys).Error; err != nil {
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
	return r.db.WithContext(ctx).Unscoped().Delete(&models.APIKey{}, "id = ?", id).Error
}
