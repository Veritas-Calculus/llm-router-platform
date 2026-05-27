package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	paymentService *billing.PaymentService
	wechatPay      *billing.WechatPayService
	alipay         *billing.AlipayService
	logger         *zap.Logger
}

type paymentTransactionResponse struct {
	ID             uuid.UUID    `json:"id"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
	OrgID          uuid.UUID    `json:"org_id"`
	UserID         uuid.UUID    `json:"user_id"`
	Type           string       `json:"type"`
	Amount         paymentMoney `json:"amount"`
	Currency       string       `json:"currency"`
	Balance        paymentMoney `json:"balance"`
	Description    string       `json:"description"`
	ReferenceID    string       `json:"reference_id"`
	IdempotencyKey *string      `json:"idempotency_key,omitempty"`
}

type paymentOrderResponse struct {
	ID            uuid.UUID    `json:"id"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	OrgID         uuid.UUID    `json:"org_id"`
	PlanID        uuid.UUID    `json:"plan_id"`
	OrderNo       string       `json:"order_no"`
	Amount        paymentMoney `json:"amount"`
	Currency      string       `json:"currency"`
	Status        string       `json:"status"`
	PaymentMethod string       `json:"payment_method"`
	ExternalID    string       `json:"external_id"`
}

type paymentMoney struct {
	decimal.Decimal
}

func newPaymentMoney(amount decimal.Decimal) paymentMoney {
	return paymentMoney{Decimal: amount.Round(models.MoneyScale)}
}

func (m paymentMoney) money() decimal.Decimal {
	return m.Round(models.MoneyScale)
}

func (m paymentMoney) MarshalJSON() ([]byte, error) {
	return []byte(m.money().String()), nil
}

func (m *paymentMoney) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		return fmt.Errorf("amount is required")
	}
	if strings.HasPrefix(raw, "\"") {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		raw = strings.TrimSpace(text)
	}
	amount, err := decimal.NewFromString(raw)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}
	m.Decimal = amount.Round(models.MoneyScale)
	return nil
}

func NewPaymentHandler(
	s *billing.PaymentService,
	wechatPay *billing.WechatPayService,
	alipay *billing.AlipayService,
	logger *zap.Logger,
) *PaymentHandler {
	return &PaymentHandler{
		paymentService: s,
		wechatPay:      wechatPay,
		alipay:         alipay,
		logger:         logger,
	}
}

func (h *PaymentHandler) CreateCheckoutSession(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)

	var req struct {
		PlanID uuid.UUID `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	url, err := h.paymentService.CreateCheckoutSession(c.Request.Context(), user.ID, req.PlanID)
	if err != nil {
		h.logger.Error("failed to create checkout session", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *PaymentHandler) CreateRechargeSession(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)

	var req struct {
		Amount        paymentMoney `json:"amount"`
		PaymentMethod string       `json:"payment_method"` // "stripe" (default), "wechat", "alipay"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}
	amount := req.Amount.money()
	if !amount.IsPositive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount must be positive"})
		return
	}

	if req.PaymentMethod == "" {
		req.PaymentMethod = "stripe"
	}

	switch req.PaymentMethod {
	case "wechat":
		if h.wechatPay == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "wechat pay is not configured"})
			return
		}
		qrCode, orderNo, err := h.wechatPay.CreateNativeOrder(c.Request.Context(), user.ID, amount, "Credit Top-up")
		if err != nil {
			h.logger.Error("failed to create wechat pay order", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create wechat pay order"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qr_code": qrCode, "order_no": orderNo, "payment_method": "wechat"})

	case "alipay":
		if h.alipay == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "alipay is not configured"})
			return
		}
		qrCode, orderNo, err := h.alipay.CreatePreCreateOrder(c.Request.Context(), user.ID, amount, "Credit Top-up")
		if err != nil {
			h.logger.Error("failed to create alipay order", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create alipay order"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qr_code": qrCode, "order_no": orderNo, "payment_method": "alipay"})

	default: // stripe
		url, err := h.paymentService.CreateRechargeSession(c.Request.Context(), user.ID, amount)
		if err != nil {
			h.logger.Error("failed to create recharge session", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create recharge session"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"url": url, "payment_method": "stripe"})
	}
}

func (h *PaymentHandler) GetMyOrders(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	orders, err := h.paymentService.GetUserOrders(c.Request.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to get user orders", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve orders"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toPaymentOrderResponses(orders)})
}

func toPaymentOrderResponses(orders []models.Order) []paymentOrderResponse {
	out := make([]paymentOrderResponse, len(orders))
	for i, order := range orders {
		out[i] = paymentOrderResponse{
			ID:            order.ID,
			CreatedAt:     order.CreatedAt,
			UpdatedAt:     order.UpdatedAt,
			OrgID:         order.OrgID,
			PlanID:        order.PlanID,
			OrderNo:       order.OrderNo,
			Amount:        newPaymentMoney(order.Amount),
			Currency:      order.Currency,
			Status:        order.Status,
			PaymentMethod: order.PaymentMethod,
			ExternalID:    order.ExternalID,
		}
	}
	return out
}

func (h *PaymentHandler) GetMyTransactions(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	txs, err := h.paymentService.GetUserTransactions(c.Request.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to get user transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": toPaymentTransactionResponses(txs)})
}

func toPaymentTransactionResponses(txs []models.Transaction) []paymentTransactionResponse {
	out := make([]paymentTransactionResponse, len(txs))
	for i, tx := range txs {
		out[i] = paymentTransactionResponse{
			ID:             tx.ID,
			CreatedAt:      tx.CreatedAt,
			UpdatedAt:      tx.UpdatedAt,
			OrgID:          tx.OrgID,
			UserID:         tx.UserID,
			Type:           tx.Type,
			Amount:         newPaymentMoney(tx.Amount),
			Currency:       tx.Currency,
			Balance:        newPaymentMoney(tx.Balance),
			Description:    tx.Description,
			ReferenceID:    tx.ReferenceID,
			IdempotencyKey: tx.IdempotencyKey,
		}
	}
	return out
}

func (h *PaymentHandler) StripeWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Derive a bounded ctx from the request so a slow DB/row lock cannot
	// hold the goroutine open past Stripe's ~30s webhook deadline.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	sigHeader := c.GetHeader("Stripe-Signature")
	if err := h.paymentService.HandleWebhook(ctx, payload, sigHeader); err != nil {
		h.logger.Error("stripe webhook failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "webhook processing failed"})
		return
	}

	c.Status(http.StatusOK)
}

// WechatPayNotify handles WeChat Pay async payment notifications.
func (h *PaymentHandler) WechatPayNotify(c *gin.Context) {
	if h.wechatPay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "wechat pay not configured"})
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "failed to read request body"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	orderNo, err := h.wechatPay.HandleNotify(ctx, payload, c.Request.Header)
	if err != nil {
		h.logger.Error("wechat pay notification failed", zap.String("error", sanitize.SafeString(err.Error())))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": "notification processing failed"})
		return
	}

	h.logger.Info("wechat pay notification processed", zap.String("order_no", sanitize.SafeString(orderNo)))
	// WeChat Pay expects a specific response format
	c.JSON(http.StatusOK, gin.H{"code": "SUCCESS", "message": ""})
}

// AlipayNotify handles Alipay async payment notifications.
func (h *PaymentHandler) AlipayNotify(c *gin.Context) {
	if h.alipay == nil {
		c.String(http.StatusNotFound, "fail")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	orderNo, err := h.alipay.HandleNotify(ctx, c.Request.Form)
	if err != nil {
		h.logger.Error("alipay notification failed",
			zap.String("error", sanitize.SafeString(err.Error())),
			zap.String("order_no", sanitize.SafeString(orderNo)))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	h.logger.Info("alipay notification processed", zap.String("order_no", sanitize.SafeString(orderNo)))
	// Alipay expects "success" as plain text response
	c.String(http.StatusOK, "success")
}
