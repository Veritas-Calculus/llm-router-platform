package handlers

import (
	"net/http"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/billing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type PlanHandler struct {
	subService *billing.SubscriptionService
	logger     *zap.Logger
}

func NewPlanHandler(s *billing.SubscriptionService, logger *zap.Logger) *PlanHandler {
	return &PlanHandler{
		subService: s,
		logger:     logger,
	}
}

func (h *PlanHandler) ListPlans(c *gin.Context) {
	plans, err := h.subService.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *PlanHandler) GetMySubscription(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	sub, err := h.subService.GetUserSubscription(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, sub)
}

// Admin only methods

func (h *PlanHandler) CreatePlan(c *gin.Context) {
	var plan models.Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.subService.CreatePlan(c.Request.Context(), &plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, plan)
}

func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan id"})
		return
	}

	var plan models.Plan
	if err := c.ShouldBindJSON(&plan); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan.ID = id

	if err := h.subService.UpdatePlan(c.Request.Context(), &plan); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, plan)
}
