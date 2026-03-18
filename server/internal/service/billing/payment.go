package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/checkout/session"
	"github.com/stripe/stripe-go/v76/webhook"
	"go.uber.org/zap"
)

// PaymentService handles payment processing.
type PaymentService struct {
	cfg         config.StripeConfig
	frontendURL string
	planRepo    repository.PlanRepo
	subRepo     repository.SubscriptionRepo
	txRepo      repository.TransactionRepo
	logger      *zap.Logger
}

func NewPaymentService(
	cfg config.StripeConfig,
	frontendURL string,
	planRepo repository.PlanRepo,
	subRepo repository.SubscriptionRepo,
	txRepo repository.TransactionRepo,
	logger *zap.Logger,
) *PaymentService {
	if cfg.Enabled {
		stripe.Key = cfg.SecretKey
	}
	return &PaymentService{
		cfg:         cfg,
		frontendURL: frontendURL,
		planRepo:    planRepo,
		subRepo:     subRepo,
		txRepo:      txRepo,
		logger:      logger,
	}
}

// CreateCheckoutSession creates a Stripe Checkout Session for a plan subscription.
func (s *PaymentService) CreateCheckoutSession(ctx context.Context, userID uuid.UUID, planID uuid.UUID) (string, error) {
	if !s.cfg.Enabled {
		return "", fmt.Errorf("payments are currently disabled")
	}

	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return "", fmt.Errorf("plan not found")
	}

	orderNo := fmt.Sprintf("ORD-%d-%s", time.Now().Unix(), uuid.New().String()[:8])

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(s.frontendURL + "/plans?payment=success&order_no=" + orderNo),
		CancelURL:  stripe.String(s.frontendURL + "/plans?payment=cancel"),
		PaymentMethodTypes: stripe.StringSlice([]string{
			"card",
		}),
		Mode: stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String(plan.Name),
						Description: stripe.String(plan.Description),
					},
					UnitAmount: stripe.Int64(int64(plan.PriceMonth * 100)),
					Recurring: &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
						Interval: stripe.String("month"),
					},
				},
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id":  userID.String(),
			"plan_id":  planID.String(),
			"order_no": orderNo,
			"type":     "subscription",
		},
		ClientReferenceID: stripe.String(orderNo),
	}

	sess, err := session.New(params)
	if err != nil {
		return "", err
	}

	// Create order record
	order := &models.Order{
		UserID:        userID,
		PlanID:        planID,
		OrderNo:       orderNo,
		Amount:        plan.PriceMonth,
		Status:        "pending",
		PaymentMethod: "stripe",
		ExternalID:    sess.ID,
	}
	if err := s.subRepo.CreateOrder(ctx, order); err != nil {
		s.logger.Error("failed to create order record", zap.Error(err))
	}

	return sess.URL, nil
}

// CreateRechargeSession creates a Stripe session for balance top-up.
func (s *PaymentService) CreateRechargeSession(ctx context.Context, userID uuid.UUID, amount float64) (string, error) {
	if !s.cfg.Enabled {
		return "", fmt.Errorf("payments are currently disabled")
	}

	orderNo := fmt.Sprintf("RECH-%d-%s", time.Now().Unix(), uuid.New().String()[:8])

	params := &stripe.CheckoutSessionParams{
		SuccessURL: stripe.String(s.frontendURL + "/billing?payment=success&order_no=" + orderNo),
		CancelURL:  stripe.String(s.frontendURL + "/billing?payment=cancel"),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("usd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Credit Top-up"),
						Description: stripe.String(fmt.Sprintf("Top up account with $%.2f", amount)),
					},
					UnitAmount: stripe.Int64(int64(amount * 100)),
				},
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"user_id":  userID.String(),
			"amount":   fmt.Sprintf("%.2f", amount),
			"type":     "recharge",
			"order_no": orderNo,
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", err
	}

	// Create order record
	order := &models.Order{
		UserID:        userID,
		OrderNo:       orderNo,
		Amount:        amount,
		Status:        "pending",
		PaymentMethod: "stripe",
		ExternalID:    sess.ID,
	}
	_ = s.subRepo.CreateOrder(ctx, order)

	return sess.URL, nil
}

// HandleWebhook processes Stripe webhooks.
func (s *PaymentService) HandleWebhook(payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEvent(payload, sigHeader, s.cfg.WebhookSecret)
	if err != nil {
		return err
	}

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			return err
		}
		return s.fulfillOrder(&session)
	}

	return nil
}

func (s *PaymentService) fulfillOrder(sess *stripe.CheckoutSession) error {
	userIDStr := sess.Metadata["user_id"]
	orderNo := sess.Metadata["order_no"]
	orderType := sess.Metadata["type"]

	userID, _ := uuid.Parse(userIDStr)

	ctx := context.Background()

	// Update order status
	order, err := s.subRepo.GetOrderByNo(ctx, orderNo)
	if err == nil {
		order.Status = "paid"
		_ = s.subRepo.UpdateOrder(ctx, order)
	}

	if orderType == "recharge" {
		amountStr := sess.Metadata["amount"]
		var amount float64
		fmt.Sscanf(amountStr, "%f", &amount)
		return s.subRepo.UpdateUserBalance(ctx, userID, amount, "recharge", "Credit Top-up via Stripe", orderNo)
	}

	// Default: Subscription fulfillment
	planIDStr := sess.Metadata["plan_id"]
	planID, _ := uuid.Parse(planIDStr)

	s.logger.Info("fulfilling subscription order", zap.String("user_id", userIDStr), zap.String("plan_id", planIDStr))

	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil {
		// Create new subscription if not exists
		sub = &models.Subscription{
			UserID: userID,
		}
	}

	sub.PlanID = planID
	sub.Status = "active"
	// Set period dates (simplified)
	sub.CurrentPeriodStart = time.Now()
	sub.CurrentPeriodEnd = time.Now().AddDate(0, 1, 0)

	if sub.ID == uuid.Nil {
		return s.subRepo.Create(ctx, sub)
	}
	return s.subRepo.Update(ctx, sub)
}

// GetUserOrders returns orders for a user.
func (s *PaymentService) GetUserOrders(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	return s.subRepo.GetOrdersByUserID(ctx, userID)
}

// GetUserTransactions returns transactions for a user.
func (s *PaymentService) GetUserTransactions(ctx context.Context, userID uuid.UUID) ([]models.Transaction, error) {
	txs, _, err := s.txRepo.GetByUserID(ctx, userID, 50, 0)
	return txs, err
}
