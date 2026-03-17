package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

// Delete permanently removes a proxy from the database.
func (r *ProxyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.Proxy{}, "id = ?", id).Error
}
