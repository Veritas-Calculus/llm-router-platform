package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrorLogRepository handles error log data access.
type ErrorLogRepository struct {
	db *gorm.DB
}

// NewErrorLogRepository creates a new error log repository.
func NewErrorLogRepository(db *gorm.DB) *ErrorLogRepository {
	return &ErrorLogRepository{db: db}
}

// Create inserts a new error log.
func (r *ErrorLogRepository) Create(ctx context.Context, log *models.ErrorLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID returns an error log by its ID.
func (r *ErrorLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ErrorLog, error) {
	var log models.ErrorLog
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}
