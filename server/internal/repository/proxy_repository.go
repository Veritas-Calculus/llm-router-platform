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
	if err := r.db.WithContext(ctx).Preload("Pool").First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

// GetByPoolID retrieves all proxies in a proxy pool.
func (r *ProxyRepository) GetByPoolID(ctx context.Context, poolID uuid.UUID) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Preload("Pool").Where("pool_id = ?", poolID).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// GetActiveByPoolID retrieves active proxies in a proxy pool.
func (r *ProxyRepository) GetActiveByPoolID(ctx context.Context, poolID uuid.UUID) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).
		Preload("Pool").
		Where("pool_id = ? AND is_active = ?", poolID, true).
		Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// GetActive retrieves all active proxies.
func (r *ProxyRepository) GetActive(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Preload("Pool").Where("is_active = ?", true).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// GetAll retrieves all proxies.
func (r *ProxyRepository) GetAll(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Preload("Pool").Find(&proxies).Error; err != nil {
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

// CreatePool inserts a proxy pool.
func (r *ProxyRepository) CreatePool(ctx context.Context, pool *models.ProxyPool) error {
	return r.db.WithContext(ctx).Create(pool).Error
}

// GetPools retrieves all proxy pools with their proxies.
func (r *ProxyRepository) GetPools(ctx context.Context) ([]models.ProxyPool, error) {
	var pools []models.ProxyPool
	if err := r.db.WithContext(ctx).Preload("Proxies").Find(&pools).Error; err != nil {
		return nil, err
	}
	return pools, nil
}

// GetPoolByID retrieves a proxy pool by ID.
func (r *ProxyRepository) GetPoolByID(ctx context.Context, id uuid.UUID) (*models.ProxyPool, error) {
	var pool models.ProxyPool
	if err := r.db.WithContext(ctx).Preload("Proxies").First(&pool, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &pool, nil
}

// UpdatePool updates a proxy pool.
func (r *ProxyRepository) UpdatePool(ctx context.Context, pool *models.ProxyPool) error {
	return r.db.WithContext(ctx).Save(pool).Error
}

// DeletePool removes a proxy pool and clears memberships first.
func (r *ProxyRepository) DeletePool(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Proxy{}).
			Where("pool_id = ?", id).
			Update("pool_id", nil).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&models.ProxyPool{}, "id = ?", id).Error
	})
}
