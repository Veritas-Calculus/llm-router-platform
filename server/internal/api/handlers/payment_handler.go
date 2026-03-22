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
	logger         *zap.Logger
}

func NewPaymentHandler(s *billing.PaymentService, logger *zap.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentService: s,
		logger:         logger,
	}
}

func (h *PaymentHandler) CreateCheckoutSession(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	
	var req struct {
		PlanID uuid.UUID `json:"plan_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url, err := h.paymentService.CreateCheckoutSession(c.Request.Context(), user.ID, req.PlanID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *PaymentHandler) CreateRechargeSession(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url, err := h.paymentService.CreateRechargeSession(c.Request.Context(), user.ID, req.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *PaymentHandler) GetMyOrders(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	orders, err := h.paymentService.GetUserOrders(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": orders})
}

func (h *PaymentHandler) GetMyTransactions(c *gin.Context) {
	user := c.MustGet("project").(*models.Project)
	txs, err := h.paymentService.GetUserTransactions(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
		h.logger.Error("webhook failed", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
