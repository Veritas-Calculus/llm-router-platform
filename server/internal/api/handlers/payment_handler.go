package handlers

import (
	"io"
	"net/http"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PaymentHandler struct {
	paymentService *billing.PaymentService
	wechatPay      *billing.WechatPayService
	alipay         *billing.AlipayService
	logger         *zap.Logger
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
		Amount        float64 `json:"amount" binding:"required,gt=0"`
		PaymentMethod string  `json:"payment_method"` // "stripe" (default), "wechat", "alipay"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
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
		qrCode, orderNo, err := h.wechatPay.CreateNativeOrder(c.Request.Context(), user.ID, req.Amount, "Credit Top-up")
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
		qrCode, orderNo, err := h.alipay.CreatePreCreateOrder(c.Request.Context(), user.ID, req.Amount, "Credit Top-up")
		if err != nil {
			h.logger.Error("failed to create alipay order", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create alipay order"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"qr_code": qrCode, "order_no": orderNo, "payment_method": "alipay"})

	default: // stripe
		url, err := h.paymentService.CreateRechargeSession(c.Request.Context(), user.ID, req.Amount)
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
	c.JSON(http.StatusOK, gin.H{"data": orders})
}

func (h *PaymentHandler) GetMyTransactions(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	txs, err := h.paymentService.GetUserTransactions(c.Request.Context(), user.ID)
	if err != nil {
		h.logger.Error("failed to get user transactions", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve transactions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": txs})
}

func (h *PaymentHandler) StripeWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	sigHeader := c.GetHeader("Stripe-Signature")
	if err := h.paymentService.HandleWebhook(payload, sigHeader); err != nil {
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

	orderNo, err := h.wechatPay.HandleNotify(payload, c.Request.Header)
	if err != nil {
		h.logger.Error("wechat pay notification failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"code": "FAIL", "message": err.Error()})
		return
	}

	h.logger.Info("wechat pay notification processed", zap.String("order_no", orderNo))
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

	orderNo, err := h.alipay.HandleNotify(c.Request.Form)
	if err != nil {
		h.logger.Error("alipay notification failed", zap.Error(err))
		c.String(http.StatusBadRequest, "fail")
		return
	}

	h.logger.Info("alipay notification processed", zap.String("order_no", orderNo))
	// Alipay expects "success" as plain text response
	c.String(http.StatusOK, "success")
}

