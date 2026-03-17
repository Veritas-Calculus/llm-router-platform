package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
