package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PlanRepository handles plan data access.
type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) Create(ctx context.Context, plan *models.Plan) error {
	return r.db.WithContext(ctx).Create(plan).Error
}

func (r *PlanRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Plan, error) {
	var plan models.Plan
	if err := r.db.WithContext(ctx).First(&plan, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *PlanRepository) GetByName(ctx context.Context, name string) (*models.Plan, error) {
	var plan models.Plan
	if err := r.db.WithContext(ctx).First(&plan, "name = ?", name).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

func (r *PlanRepository) GetAll(ctx context.Context) ([]models.Plan, error) {
	var plans []models.Plan
	if err := r.db.WithContext(ctx).Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *PlanRepository) GetActive(ctx context.Context) ([]models.Plan, error) {
	var plans []models.Plan
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&plans).Error; err != nil {
		return nil, err
	}
	return plans, nil
}

func (r *PlanRepository) Update(ctx context.Context, plan *models.Plan) error {
	return r.db.WithContext(ctx).Save(plan).Error
}

func (r *PlanRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Plan{}, "id = ?", id).Error
}

// SubscriptionRepository handles subscription data access.
type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *models.Subscription) error {
	return r.db.WithContext(ctx).Create(sub).Error
}

// GetByUserID retrieves a user's active subscription.
func (r *SubscriptionRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Subscription, error) {
	var sub models.Subscription
	if err := r.db.WithContext(ctx).Preload("Plan").First(&sub, "org_id = ?", userID).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetByStripeCustomerID retrieves a subscription by its associated Stripe Customer ID.
func (r *SubscriptionRepository) GetByStripeCustomerID(ctx context.Context, customerID string) (*models.Subscription, error) {
	var sub models.Subscription
	err := r.db.WithContext(ctx).Where("stripe_customer_id = ?", customerID).First(&sub).Error
	return &sub, err
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *models.Subscription) error {
	return r.db.WithContext(ctx).
		Select("plan_id", "status", "current_period_start", "current_period_end", "cancel_at_period_end", "stripe_customer_id", "stripe_subscription_id", "updated_at").
		Save(sub).Error
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.Subscription{}, "id = ?", id).Error
}

// Order methods

func (r *SubscriptionRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	return r.db.WithContext(ctx).Create(order).Error
}

func (r *SubscriptionRepository) GetOrderByNo(ctx context.Context, orderNo string) (*models.Order, error) {
	var order models.Order
	if err := r.db.WithContext(ctx).First(&order, "order_no = ?", orderNo).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *SubscriptionRepository) GetOrdersByUserID(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	var orders []models.Order
	if err := r.db.WithContext(ctx).Where("org_id = ?", userID).Order("created_at DESC").Find(&orders).Error; err != nil {
		return nil, err
	}
	return orders, nil
}

func (r *SubscriptionRepository) UpdateOrder(ctx context.Context, order *models.Order) error {
	return r.db.WithContext(ctx).Save(order).Error
}

func (r *SubscriptionRepository) UpdateUserBalance(ctx context.Context, userID uuid.UUID, amount float64, txType, description, referenceID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance += amount
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID, // Using user ID as org ID temporarily
			Type:        txType,
			Amount:      amount,
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		return tx.Create(transaction).Error
	})
}

// TransactionRepository handles transaction data access.
type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, tx *models.Transaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *TransactionRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Transaction, int64, error) {
	var txs []models.Transaction
	var total int64

	query := r.db.WithContext(ctx).Model(&models.Transaction{}).Where("org_id = ?", userID)
	
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&txs).Error; err != nil {
		return nil, 0, err
	}

	return txs, total, nil
}
