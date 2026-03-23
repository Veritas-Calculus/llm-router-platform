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
	"github.com/stripe/stripe-go/v76/billingportal/session"
	checkoutSession "github.com/stripe/stripe-go/v76/checkout/session"
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

	sess, err := checkoutSession.New(params)
	if err != nil {
		return "", err
	}

	// Create order record
	order := &models.Order{
		OrgID:         userID,
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

	sess, err := checkoutSession.New(params)
	if err != nil {
		return "", err
	}

	// Create order record
	order := &models.Order{
		OrgID:         userID,
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
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return err
		}
		return s.fulfillOrder(&sess)
	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionUpdated(&sub)
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionDeleted(&sub)
	}

	return nil
}

func (s *PaymentService) fulfillOrder(sess *stripe.CheckoutSession) error {
	userIDStr := sess.Metadata["user_id"]
	orderNo := sess.Metadata["order_no"]
	orderType := sess.Metadata["type"]

	userID, _ := uuid.Parse(userIDStr)

	ctx := context.Background()

	// Idempotency: if the order is already fulfilled, skip re-processing.
	// Stripe may retry webhooks, so this prevents duplicate balance top-ups.
	order, err := s.subRepo.GetOrderByNo(ctx, orderNo)
	if err == nil {
		if order.Status == "paid" {
			s.logger.Info("order already fulfilled, skipping duplicate webhook",
				zap.String("order_no", orderNo))
			return nil
		}
		order.Status = "paid"
		_ = s.subRepo.UpdateOrder(ctx, order)
	}

	if orderType == "recharge" {
		amountStr := sess.Metadata["amount"]
		var amount float64
		_, _ = fmt.Sscanf(amountStr, "%f", &amount)
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
			OrgID: userID,
		}
	}

	sub.PlanID = planID
	sub.Status = "active"
	// Set period dates from stripe object if available, simplified otherwise
	sub.CurrentPeriodStart = time.Now()
	sub.CurrentPeriodEnd = time.Now().AddDate(0, 1, 0)

	if sess.Customer != nil {
		sub.StripeCustomerID = sess.Customer.ID
	}
	if sess.Subscription != nil {
		sub.StripeSubscriptionID = sess.Subscription.ID
	}

	if sub.ID == uuid.Nil {
		return s.subRepo.Create(ctx, sub)
	}
	return s.subRepo.Update(ctx, sub)
}

// CreatePortalSession creates a Stripe billing portal session.
func (s *PaymentService) CreatePortalSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if !s.cfg.Enabled {
		return "", fmt.Errorf("payments are currently disabled")
	}

	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil || sub.StripeCustomerID == "" {
		return "", fmt.Errorf("no active subscription associated with this account")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID),
		ReturnURL: stripe.String(s.frontendURL + "/billing"),
	}

	sess, err := session.New(params)
	if err != nil {
		return "", err
	}

	return sess.URL, nil
}

func (s *PaymentService) handleSubscriptionUpdated(stripeSub *stripe.Subscription) error {
	ctx := context.Background()
	sub, err := s.subRepo.GetByStripeCustomerID(ctx, stripeSub.Customer.ID)
	if err != nil {
		s.logger.Warn("webhook updated subscription but no local sub found", zap.String("customer_id", stripeSub.Customer.ID))
		return nil // Not tracking this customer
	}

	// Update the period end
	sub.CurrentPeriodStart = time.Unix(stripeSub.CurrentPeriodStart, 0)
	sub.CurrentPeriodEnd = time.Unix(stripeSub.CurrentPeriodEnd, 0)
	sub.Status = string(stripeSub.Status)
	sub.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd

	// Map the Stripe price to a local PlanID
	if len(stripeSub.Items.Data) > 0 {
		priceAmount := float64(stripeSub.Items.Data[0].Price.UnitAmount) / 100.0
		plans, err := s.planRepo.GetActive(ctx)
		if err == nil {
			for _, p := range plans {
				if p.PriceMonth == priceAmount {
					sub.PlanID = p.ID
					s.logger.Info("mapped stripe subscription to local plan", zap.String("plan_name", p.Name))
					break
				}
			}
		}
	}

	s.logger.Info("updated subscription via webhook", zap.String("sub_id", sub.ID.String()), zap.String("status", sub.Status))
	return s.subRepo.Update(ctx, sub)
}

func (s *PaymentService) handleSubscriptionDeleted(stripeSub *stripe.Subscription) error {
	ctx := context.Background()
	sub, err := s.subRepo.GetByStripeCustomerID(ctx, stripeSub.Customer.ID)
	if err != nil {
		return nil
	}

	sub.Status = "canceled"
	s.logger.Info("canceled subscription via webhook", zap.String("sub_id", sub.ID.String()))
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
