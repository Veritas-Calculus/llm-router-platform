// Package announcement provides announcement management services.
package announcement

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service handles announcement operations.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new announcement service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create creates a new announcement.
func (s *Service) Create(ctx context.Context, a *models.Announcement) error {
	return s.db.WithContext(ctx).Create(a).Error
}

// Update updates an existing announcement.
func (s *Service) Update(ctx context.Context, a *models.Announcement) error {
	return s.db.WithContext(ctx).Save(a).Error
}

// Delete deletes an announcement by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.Announcement{}, "id = ?", id).Error
}

// GetByID retrieves an announcement by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Announcement, error) {
	var a models.Announcement
	if err := s.db.WithContext(ctx).First(&a, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// GetAll retrieves all announcements (admin).
func (s *Service) GetAll(ctx context.Context) ([]models.Announcement, error) {
	var announcements []models.Announcement
	err := s.db.WithContext(ctx).Order("priority DESC, created_at DESC").Find(&announcements).Error
	return announcements, err
}

// GetActive retrieves currently active announcements (user-facing).
func (s *Service) GetActive(ctx context.Context) ([]models.Announcement, error) {
	var announcements []models.Announcement
	now := time.Now()
	err := s.db.WithContext(ctx).
		Where("is_active = ? AND (starts_at IS NULL OR starts_at <= ?) AND (ends_at IS NULL OR ends_at >= ?)",
			true, now, now).
		Order("priority DESC, created_at DESC").
		Find(&announcements).Error
	return announcements, err
}
