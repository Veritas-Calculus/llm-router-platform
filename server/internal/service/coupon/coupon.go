// Package coupon provides coupon management services.
package coupon

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service handles coupon operations.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new coupon service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create creates a new coupon.
func (s *Service) Create(ctx context.Context, c *models.Coupon) error {
	return s.db.WithContext(ctx).Create(c).Error
}

// Update updates an existing coupon.
func (s *Service) Update(ctx context.Context, c *models.Coupon) error {
	return s.db.WithContext(ctx).Save(c).Error
}

// Delete deletes a coupon by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.Coupon{}, "id = ?", id).Error
}

// GetByID retrieves a coupon by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Coupon, error) {
	var c models.Coupon
	if err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// GetAll retrieves all coupons (admin).
func (s *Service) GetAll(ctx context.Context) ([]models.Coupon, error) {
	var coupons []models.Coupon
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&coupons).Error
	return coupons, err
}

// GetByCode retrieves a coupon by its code.
func (s *Service) GetByCode(ctx context.Context, code string) (*models.Coupon, error) {
	var c models.Coupon
	if err := s.db.WithContext(ctx).Where("code = ?", code).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}
