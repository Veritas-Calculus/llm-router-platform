package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"gorm.io/gorm"
)

// ConfigRepository handles system configuration data access.
type ConfigRepository struct {
	db *gorm.DB
}

func NewConfigRepository(db *gorm.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

func (r *ConfigRepository) Get(ctx context.Context, key string) (*models.SystemConfig, error) {
	var cfg models.SystemConfig
	if err := r.db.WithContext(ctx).First(&cfg, "key = ?", key).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *ConfigRepository) Set(ctx context.Context, cfg *models.SystemConfig) error {
	var existing models.SystemConfig
	err := r.db.WithContext(ctx).First(&existing, "key = ?", cfg.Key).Error
	if err != nil {
		// Not found — create
		return r.db.WithContext(ctx).Create(cfg).Error
	}
	// Found — update
	existing.Value = cfg.Value
	existing.Description = cfg.Description
	existing.Category = cfg.Category
	existing.IsSecret = cfg.IsSecret
	return r.db.WithContext(ctx).Save(&existing).Error
}

func (r *ConfigRepository) GetByCategory(ctx context.Context, category string) ([]models.SystemConfig, error) {
	var configs []models.SystemConfig
	if err := r.db.WithContext(ctx).Where("category = ?", category).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}

func (r *ConfigRepository) Delete(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Where("key = ?", key).Delete(&models.SystemConfig{}).Error
}
