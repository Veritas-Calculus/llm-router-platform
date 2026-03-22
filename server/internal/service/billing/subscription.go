package billing

import (
	"context"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SubscriptionService handles plans and subscriptions.
type SubscriptionService struct {
	planRepo    repository.PlanRepo
	subRepo     repository.SubscriptionRepo
	usageRepo   repository.UsageLogRepo
	logger      *zap.Logger
}

func NewSubscriptionService(
	planRepo repository.PlanRepo,
	subRepo repository.SubscriptionRepo,
	usageRepo repository.UsageLogRepo,
	logger *zap.Logger,
) *SubscriptionService {
	return &SubscriptionService{
		planRepo:  planRepo,
		subRepo:   subRepo,
		usageRepo: usageRepo,
		logger:    logger,
	}
}

// GetUserSubscription returns the user's active subscription.
// If none exists, it creates a default "Free" subscription.
func (s *SubscriptionService) GetUserSubscription(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err == nil {
		return sub, nil
	}

	// Create default free subscription
	freePlan, err := s.planRepo.GetByName(ctx, "Free")
	if err != nil {
		return nil, err
	}

	newSub := &models.Subscription{
		OrgID:             userID,
		PlanID:            freePlan.ID,
		Status:            "active",
		CurrentPeriodStart: time.Now(),
		CurrentPeriodEnd:   time.Now().AddDate(0, 1, 0), // 1 month
	}

	if err := s.subRepo.Create(ctx, newSub); err != nil {
		return nil, err
	}

	newSub.Plan = *freePlan
	return newSub, nil
}

// CheckQuota verifies if the user has remaining quota based on their subscription.
func (s *SubscriptionService) CheckQuota(ctx context.Context, userID uuid.UUID) (bool, string, error) {
	sub, err := s.GetUserSubscription(ctx, userID)
	if err != nil {
		return false, "unable to verify subscription", err
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

func (s *SubscriptionService) CreatePlan(ctx context.Context, plan *models.Plan) error {
	return s.planRepo.Create(ctx, plan)
}

func (s *SubscriptionService) UpdatePlan(ctx context.Context, plan *models.Plan) error {
	return s.planRepo.Update(ctx, plan)
}
