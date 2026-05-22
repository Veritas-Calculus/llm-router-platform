package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
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
	redis       *redis.Client
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

// WithRedis wires a Redis client for Stripe webhook event-ID idempotency.
// Optional: if redis is nil the order-status check still provides a
// secondary defense, but cross-event replay protection is unavailable.
func (s *PaymentService) WithRedis(rdb *redis.Client) *PaymentService {
	s.redis = rdb
	return s
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
	if !plan.IsActive {
		return "", fmt.Errorf("plan is not available")
	}
	if plan.PriceMonth <= 0 {
		return "", fmt.Errorf("free plans do not require checkout")
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
		SuccessURL:         stripe.String(s.frontendURL + "/billing?payment=success&order_no=" + orderNo),
		CancelURL:          stripe.String(s.frontendURL + "/billing?payment=cancel"),
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		Mode:               stripe.String(string(stripe.CheckoutSessionModePayment)),
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
//
// Idempotency layers (in order):
//  1. Stripe signature on the raw payload (rejects forged events).
//  2. Redis SETNX on event.ID (rejects redelivered events from the same
//     dispatch — Stripe retries up to ~3 days).
//  3. Order.Status == "paid" check inside fulfillOrder (last-resort guard
//     against Redis unavailability).
//
// ctx is derived from the inbound HTTP request so a slow DB or row lock does
// not pin the webhook handler past the request's deadline.
func (s *PaymentService) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	event, err := webhook.ConstructEvent(payload, sigHeader, s.cfg.WebhookSecret)
	if err != nil {
		return err
	}

	if s.redis != nil && event.ID != "" {
		setCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		set, err := s.redis.SetNX(setCtx, fmt.Sprintf("stripe:event:%s", event.ID), "1", 24*time.Hour).Result()
		cancel()
		if err == nil && !set {
			s.logger.Info("stripe event already processed, skipping",
				zap.String("event_id", event.ID),
				zap.String("event_type", string(event.Type)))
			return nil
		}
	}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return err
		}
		return s.fulfillOrder(ctx, &sess)
	case "customer.subscription.updated":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionUpdated(ctx, &sub)
	case "customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return err
		}
		return s.handleSubscriptionDeleted(ctx, &sub)
	}

	return nil
}

func (s *PaymentService) fulfillOrder(ctx context.Context, sess *stripe.CheckoutSession) error {
	userIDStr := sess.Metadata["user_id"]
	orderNo := sess.Metadata["order_no"]
	orderType := sess.Metadata["type"]

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.logger.Error("stripe webhook: invalid user_id metadata, refusing to fulfill",
			zap.String("order_no", orderNo),
			zap.String("user_id_raw", userIDStr),
			zap.Error(err))
		return fmt.Errorf("invalid user_id in stripe metadata: %w", err)
	}

	// Idempotency: if the order is already fulfilled, skip re-processing.
	// Stripe may retry webhooks, so this prevents duplicate balance top-ups.
	order, err := s.subRepo.GetOrderByNo(ctx, orderNo)
	if err == nil {
		if order.Status == "paid" {
			s.logger.Info("order already fulfilled, skipping duplicate webhook",
				zap.String("order_no", orderNo))
			return nil
		}
	}
	markOrderPaid := func() error {
		if err != nil || order == nil {
			return nil
		}
		order.Status = "paid"
		return s.subRepo.UpdateOrder(ctx, order)
	}

	if orderType == "recharge" {
		// Read the authoritative amount from the signed webhook event payload,
		// not from metadata. Stripe reports AmountTotal in the smallest
		// currency subunit (cents for USD) so divide by 100.
		if sess.AmountTotal <= 0 {
			s.logger.Error("stripe recharge with non-positive amount_total, refusing to credit",
				zap.String("order_no", orderNo),
				zap.Int64("amount_total", sess.AmountTotal))
			return fmt.Errorf("invalid stripe amount")
		}
		amount := float64(sess.AmountTotal) / 100.0
		// Cross-check against the order amount we recorded at checkout creation.
		// A divergence indicates a price drift or tampered metadata; refuse.
		if order != nil && math.Abs(amount-order.Amount) > 0.01 {
			recordPaymentAmountMismatch("stripe")
			s.logger.Error("stripe recharge amount mismatch — refusing to credit",
				zap.String("order_no", orderNo),
				zap.Float64("session_amount", amount),
				zap.Float64("order_amount", order.Amount))
			return fmt.Errorf("amount mismatch: session=%.2f order=%.2f", amount, order.Amount)
		}
		if err := s.subRepo.UpdateUserBalance(ctx, userID, amount, "recharge", "Credit Top-up via Stripe", orderNo); err != nil {
			return err
		}
		return markOrderPaid()
	}

	// Default: Subscription fulfillment
	planIDStr := sess.Metadata["plan_id"]
	planID, err := uuid.Parse(planIDStr)
	if err != nil {
		s.logger.Error("stripe webhook: invalid plan_id metadata, refusing to fulfill",
			zap.String("order_no", orderNo),
			zap.String("plan_id_raw", planIDStr),
			zap.Error(err))
		return fmt.Errorf("invalid plan_id in stripe metadata: %w", err)
	}

	// Cross-check the paid amount against the plan price. Metadata is not
	// individually signed — the whole session payload is — so the plan_id
	// could in principle drift from the price the user actually paid (e.g. a
	// tampered checkout-session creation paths the user through a cheap plan
	// then activates an expensive one). Refuse if amount and plan disagree.
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		s.logger.Error("stripe webhook: plan_id not found, refusing to fulfill",
			zap.String("order_no", orderNo),
			zap.String("plan_id", planIDStr),
			zap.Error(err))
		return fmt.Errorf("plan not found: %w", err)
	}
	if sess.AmountTotal <= 0 {
		s.logger.Error("stripe subscription with non-positive amount_total, refusing to fulfill",
			zap.String("order_no", orderNo),
			zap.Int64("amount_total", sess.AmountTotal))
		return fmt.Errorf("invalid stripe amount")
	}
	sessionAmount := float64(sess.AmountTotal) / 100.0
	if math.Abs(sessionAmount-plan.PriceMonth) > 0.01 {
		recordPaymentAmountMismatch("stripe")
		s.logger.Error("stripe subscription amount mismatch — refusing to fulfill",
			zap.String("order_no", orderNo),
			zap.String("plan_id", planIDStr),
			zap.Float64("session_amount", sessionAmount),
			zap.Float64("plan_price", plan.PriceMonth))
		return fmt.Errorf("amount mismatch: session=%.2f plan=%.2f", sessionAmount, plan.PriceMonth)
	}

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
		customerID := sess.Customer.ID
		sub.StripeCustomerID = &customerID
	}
	if sess.Subscription != nil {
		subscriptionID := sess.Subscription.ID
		sub.StripeSubscriptionID = &subscriptionID
	}

	if sub.ID == uuid.Nil {
		err = s.subRepo.Create(ctx, sub)
	} else {
		err = s.subRepo.Update(ctx, sub)
	}
	if err != nil {
		return err
	}
	return markOrderPaid()
}

// CreatePortalSession creates a Stripe billing portal session.
func (s *PaymentService) CreatePortalSession(ctx context.Context, userID uuid.UUID) (string, error) {
	if !s.cfg.Enabled {
		return "", fmt.Errorf("payments are currently disabled")
	}

	sub, err := s.subRepo.GetByUserID(ctx, userID)
	if err != nil || sub.StripeCustomerID == nil || *sub.StripeCustomerID == "" {
		return "", fmt.Errorf("no active subscription associated with this account")
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(*sub.StripeCustomerID),
		ReturnURL: stripe.String(s.frontendURL + "/billing"),
	}

	sess, err := session.New(params)
	if err != nil {
		return "", err
	}

	return sess.URL, nil
}

func (s *PaymentService) handleSubscriptionUpdated(ctx context.Context, stripeSub *stripe.Subscription) error {
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

func (s *PaymentService) handleSubscriptionDeleted(ctx context.Context, stripeSub *stripe.Subscription) error {
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
