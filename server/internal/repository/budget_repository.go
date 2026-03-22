// Package repository provides database access layer.
package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BudgetRepository handles budget data access.
type BudgetRepository struct {
	db *gorm.DB
}

// NewBudgetRepository creates a new budget repository.
func NewBudgetRepository(db *gorm.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

// Upsert creates or updates a budget for a user (one budget per user).
func (r *BudgetRepository) Upsert(ctx context.Context, budget *models.Budget) error {
	var existing models.Budget
	err := r.db.WithContext(ctx).Where("org_id = ?", budget.OrgID).First(&existing).Error
	if err != nil {
		// Not found — create
		return r.db.WithContext(ctx).Create(budget).Error
	}
	// Found — update
	existing.MonthlyLimitUSD = budget.MonthlyLimitUSD
	existing.AlertThreshold = budget.AlertThreshold
	existing.IsActive = budget.IsActive
	existing.WebhookURL = budget.WebhookURL
	existing.Email = budget.Email
	existing.APIKeyID = budget.APIKeyID
	return r.db.WithContext(ctx).Save(&existing).Error
}

// GetByUserID retrieves the budget for a user.
func (r *BudgetRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Budget, error) {
	var budget models.Budget
	if err := r.db.WithContext(ctx).Where("org_id = ?", userID).First(&budget).Error; err != nil {
		return nil, err
	}
	return &budget, nil
}

// DeleteByUserID removes the budget for a user.
func (r *BudgetRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("org_id = ?", userID).Delete(&models.Budget{}).Error
}
