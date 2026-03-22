package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RoutingRuleRepository struct {
	db *gorm.DB
}

// NewRoutingRuleRepository creates a new RoutingRuleRepo.
func NewRoutingRuleRepository(db *gorm.DB) RoutingRuleRepo {
	return &RoutingRuleRepository{db: db}
}

func (r *RoutingRuleRepository) Create(ctx context.Context, rule *models.RoutingRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *RoutingRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RoutingRule, error) {
	var rule models.RoutingRule
	if err := r.db.WithContext(ctx).Preload("TargetProvider").Preload("FallbackProvider").First(&rule, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *RoutingRuleRepository) GetAll(ctx context.Context) ([]models.RoutingRule, error) {
	var rules []models.RoutingRule
	// Order by Priority DESC initially, then fallback to CreatedAt
	err := r.db.WithContext(ctx).Preload("TargetProvider").Preload("FallbackProvider").Order("priority DESC, created_at DESC").Find(&rules).Error
	return rules, err
}

func (r *RoutingRuleRepository) GetActive(ctx context.Context) ([]models.RoutingRule, error) {
	var rules []models.RoutingRule
	err := r.db.WithContext(ctx).Where("is_enabled = ?", true).Preload("TargetProvider").Preload("FallbackProvider").Order("priority DESC").Find(&rules).Error
	return rules, err
}

func (r *RoutingRuleRepository) Update(ctx context.Context, rule *models.RoutingRule) error {
	return r.db.WithContext(ctx).Save(rule).Error
}

func (r *RoutingRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.RoutingRule{}, "id = ?", id).Error
}
