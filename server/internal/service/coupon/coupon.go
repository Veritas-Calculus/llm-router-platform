// Package coupon provides coupon management services.
package coupon

import (
	"context"
	"fmt"
	"strings"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
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
	if err := normalizeAndValidate(c); err != nil {
		return err
	}
	return s.db.WithContext(ctx).Create(c).Error
}

// Update updates an existing coupon.
func (s *Service) Update(ctx context.Context, c *models.Coupon) error {
	if err := normalizeAndValidate(c); err != nil {
		return err
	}
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
	if err := s.db.WithContext(ctx).Where("code = ?", strings.ToUpper(strings.TrimSpace(code))).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func normalizeAndValidate(c *models.Coupon) error {
	c.Code = strings.ToUpper(strings.TrimSpace(c.Code))
	c.Name = strings.TrimSpace(c.Name)
	c.Type = strings.ToLower(strings.TrimSpace(c.Type))

	if c.Code == "" {
		return fmt.Errorf("coupon code is required")
	}
	if len(c.Code) > 32 {
		return fmt.Errorf("coupon code must be 32 characters or fewer")
	}
	if c.Name == "" {
		return fmt.Errorf("coupon name is required")
	}
	if c.Type != "percent" && c.Type != "fixed" {
		return fmt.Errorf("coupon type must be percent or fixed")
	}
	c.DiscountValue = c.DiscountValue.Round(models.MoneyScale)
	c.MinAmount = c.MinAmount.Round(models.MoneyScale)
	if !c.DiscountValue.IsPositive() {
		return fmt.Errorf("discount value must be positive")
	}
	if c.Type == "percent" && c.DiscountValue.Cmp(decimal.NewFromInt(100)) > 0 {
		return fmt.Errorf("percentage discount cannot exceed 100")
	}
	if c.MinAmount.IsNegative() || c.MaxUses < 0 || c.MaxUsesPerUser < 0 {
		return fmt.Errorf("coupon limits cannot be negative")
	}
	return nil
}
