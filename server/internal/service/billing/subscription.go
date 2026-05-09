package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubscriptionService handles plans and subscriptions.
type SubscriptionService struct {
	planRepo  repository.PlanRepo
	subRepo   repository.SubscriptionRepo
	usageRepo repository.UsageLogRepo
	db        *gorm.DB
	logger    *zap.Logger
}

func NewSubscriptionService(
	planRepo repository.PlanRepo,
	subRepo repository.SubscriptionRepo,
	usageRepo repository.UsageLogRepo,
	db *gorm.DB,
	logger *zap.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		planRepo:  planRepo,
		subRepo:   subRepo,
		usageRepo: usageRepo,
		db:        db,
		logger:    logger,
	}
}

// GetUserSubscription returns the org's current subscription, if it has one.
func (s *SubscriptionService) GetUserSubscription(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err == nil {
		return sub, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

// CheckQuota verifies if the user has remaining quota based on their subscription.
func (s *SubscriptionService) CheckQuota(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	sub, err := s.GetUserSubscription(ctx, userID)
	if err != nil {
		return false, "unable to verify subscription", err
	}
	if sub == nil {
		return true, "", nil
	}
	if sub.Status != "active" && sub.Status != "trialing" {
		return false, "subscription is not active", nil
	}

	if sub.Plan.TokenLimit == 0 {
		return true, "", nil // Unlimited
	}

	// Calculate usage in current period
	summary, err := s.usageRepo.AggregateByTimeRange(ctx, &userID, nil, nil, sub.CurrentPeriodStart, time.Now())
	if err != nil {
		return false, "unable to verify usage", err
	}

	if summary.TotalTokens >= sub.Plan.TokenLimit {
		return false, "monthly token limit reached for your plan", nil
	}

	return true, "", nil
}

// CRUD for Plans (Admin only)

func (s *SubscriptionService) ListPlans(ctx context.Context) ([]models.Plan, error) {
	return s.planRepo.GetAll(ctx)
}

// GetQuotaUsage returns the total token usage for the given org in its current subscription period.
func (s *SubscriptionService) GetQuotaUsage(ctx context.Context, orgID uuid.UUID) (int64, error) {
	sub, err := s.GetUserSubscription(ctx, orgID)
	if err != nil {
		return 0, err
	}
	if sub == nil {
		return 0, nil
	}
	summary, err := s.usageRepo.AggregateByTimeRange(ctx, &orgID, nil, nil, sub.CurrentPeriodStart, time.Now())
	if err != nil {
		return 0, err
	}
	if summary == nil {
		return 0, nil
	}
	return summary.TotalTokens, nil
}

// ChangePlan switches the org's current subscription to the target plan.
// Paid plans are charged from the user's prepaid balance. External checkout is
// intentionally separate in PaymentService.
func (s *SubscriptionService) ChangePlan(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, planID uuid.UUID) (*models.Subscription, error) {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return nil, fmt.Errorf("plan not found")
	}
	if !plan.IsActive {
		return nil, fmt.Errorf("plan is not available")
	}

	if plan.PriceMonth <= 0 {
		return s.changeFreePlan(ctx, orgID, planID, plan)
	}

	if s.db == nil {
		return nil, fmt.Errorf("balance payments are not available")
	}

	return s.changePaidPlanWithBalance(ctx, orgID, userID, plan)
}

func (s *SubscriptionService) changeFreePlan(ctx context.Context, orgID uuid.UUID, planID uuid.UUID, plan *models.Plan) (*models.Subscription, error) {
	sub, err := s.GetUserSubscription(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		sub = &models.Subscription{
			OrgID:              orgID,
			PlanID:             planID,
			Status:             "active",
			CurrentPeriodStart: time.Now(),
			CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0),
		}
		if err := s.subRepo.Create(ctx, sub); err != nil {
			return nil, err
		}
		sub.Plan = *plan
		return sub, nil
	}

	// Update plan and reset billing period
	sub.PlanID = planID
	sub.Status = "active"
	sub.CurrentPeriodStart = time.Now()
	sub.CurrentPeriodEnd = time.Now().AddDate(0, 1, 0)
	sub.CancelAtPeriodEnd = false

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return nil, err
	}

	// Re-fetch to get a clean copy with the correct Plan preloaded,
	// avoiding GORM association pitfalls.
	updated, err := s.subRepo.GetByUserID(ctx, sub.OrgID)
	if err != nil {
		// Fallback: attach the Plan we already have
		sub.Plan = *plan
		return sub, nil
	}
	return updated, nil
}

func (s *SubscriptionService) changePaidPlanWithBalance(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, plan *models.Plan) (*models.Subscription, error) {
	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	now := time.Now()
	orderNo := fmt.Sprintf("ORD-BAL-%d-%s", now.Unix(), uuid.New().String()[:8])
	amount := roundCost(plan.PriceMonth)
	var updated models.Subscription

	err := s.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		var existing models.Subscription
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Plan").First(&existing, "org_id = ?", orgID).Error
		if err == nil && existing.PlanID == plan.ID && existing.Status == "active" {
			updated = existing
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		if user.Balance < amount {
			return fmt.Errorf("insufficient balance")
		}

		user.Balance = roundCost(user.Balance - amount)
		if err := tx.Model(&user).Update("balance", user.Balance).Error; err != nil {
			return err
		}

		order := &models.Order{
			OrgID:         orgID,
			PlanID:        plan.ID,
			OrderNo:       orderNo,
			Amount:        amount,
			Status:        "paid",
			PaymentMethod: "balance",
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       orgID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      -amount,
			Balance:     user.Balance,
			Description: "Subscription plan: " + plan.Name,
			ReferenceID: orderNo,
		}
		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		sub := existing
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sub = models.Subscription{OrgID: orgID}
		}
		sub.PlanID = plan.ID
		sub.Status = "active"
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
		sub.CancelAtPeriodEnd = false

		if sub.ID == uuid.Nil {
			if err := tx.Create(&sub).Error; err != nil {
				return err
			}
		} else {
			updates := map[string]any{
				"plan_id":              sub.PlanID,
				"status":               sub.Status,
				"current_period_start": sub.CurrentPeriodStart,
				"current_period_end":   sub.CurrentPeriodEnd,
				"cancel_at_period_end": sub.CancelAtPeriodEnd,
				"updated_at":           now,
			}
			if err := tx.Model(&models.Subscription{}).Where("id = ?", sub.ID).Updates(updates).Error; err != nil {
				return err
			}
		}

		if err := tx.Preload("Plan").First(&updated, "id = ?", sub.ID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if txCtx.Err() != nil {
			recordLockTimeout("change_plan_balance")
		}
		return nil, err
	}
	if updated.Plan.ID == uuid.Nil {
		updated.Plan = *plan
	}
	return &updated, nil
}

func (s *SubscriptionService) CreatePlan(ctx context.Context, plan *models.Plan) error {
	return s.planRepo.Create(ctx, plan)
}

func (s *SubscriptionService) UpdatePlan(ctx context.Context, plan *models.Plan) error {
	return s.planRepo.Update(ctx, plan)
}
