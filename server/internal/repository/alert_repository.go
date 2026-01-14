// Package repository provides database access layer.
package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"llm-router-platform/internal/models"
)

// AlertRepository handles alert data access.
type AlertRepository struct {
	db *gorm.DB
}

// NewAlertRepository creates a new alert repository.
func NewAlertRepository(db *gorm.DB) *AlertRepository {
	return &AlertRepository{db: db}
}

// Create inserts a new alert.
func (r *AlertRepository) Create(ctx context.Context, alert *models.Alert) error {
	return r.db.WithContext(ctx).Create(alert).Error
}

// GetByID retrieves an alert by ID.
func (r *AlertRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Alert, error) {
	var alert models.Alert
	if err := r.db.WithContext(ctx).First(&alert, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &alert, nil
}

// GetByStatus retrieves alerts by status with pagination.
func (r *AlertRepository) GetByStatus(ctx context.Context, status string, offset, limit int) ([]models.Alert, int64, error) {
	var alerts []models.Alert
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Alert{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&alerts).Error; err != nil {
		return nil, 0, err
	}

	return alerts, total, nil
}

// GetByTarget retrieves alerts for a specific target.
func (r *AlertRepository) GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.Alert, error) {
	var alerts []models.Alert
	if err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("created_at DESC").
		Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

// Update updates an alert.
func (r *AlertRepository) Update(ctx context.Context, alert *models.Alert) error {
	return r.db.WithContext(ctx).Save(alert).Error
}

// AlertConfigRepository handles alert configuration data access.
type AlertConfigRepository struct {
	db *gorm.DB
}

// NewAlertConfigRepository creates a new alert config repository.
func NewAlertConfigRepository(db *gorm.DB) *AlertConfigRepository {
	return &AlertConfigRepository{db: db}
}

// Create inserts a new alert config.
func (r *AlertConfigRepository) Create(ctx context.Context, config *models.AlertConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

// GetByTarget retrieves alert config for a target.
func (r *AlertConfigRepository) GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID) (*models.AlertConfig, error) {
	var config models.AlertConfig
	if err := r.db.WithContext(ctx).
		First(&config, "target_type = ? AND target_id = ?", targetType, targetID).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// Update updates an alert config.
func (r *AlertConfigRepository) Update(ctx context.Context, config *models.AlertConfig) error {
	return r.db.WithContext(ctx).Save(config).Error
}

// GetAll retrieves all alert configs.
func (r *AlertConfigRepository) GetAll(ctx context.Context) ([]models.AlertConfig, error) {
	var configs []models.AlertConfig
	if err := r.db.WithContext(ctx).Find(&configs).Error; err != nil {
		return nil, err
	}
	return configs, nil
}
