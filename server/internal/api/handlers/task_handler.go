// Package handlers provides HTTP request handlers.
// This file contains async task management endpoints.
package handlers

import (
	"net/http"
	"strconv"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/service/task"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TaskHandler handles async task management endpoints.
type TaskHandler struct {
	taskService *task.Service
	logger      *zap.Logger
}

// NewTaskHandler creates a new task handler.
func NewTaskHandler(taskService *task.Service, logger *zap.Logger) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		logger:      logger,
	}
}

// CreateTaskRequest represents a request to create an async task.
type CreateTaskRequest struct {
	Type       string `json:"type" binding:"required"`
	Input      string `json:"input" binding:"required"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// CreateTask creates a new async task.
func (h *TaskHandler) CreateTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate task type
	validTypes := map[string]bool{
		"tts":            true,
		"batch_tts":      true,
		"video_analysis": true,
		"batch_image":    true,
	}
	if !validTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid task type, must be one of: tts, batch_tts, video_analysis, batch_image"})
		return
	}

	userObj := c.MustGet("user").(*models.User)

	t, err := h.taskService.CreateTask(c.Request.Context(), userObj.ID, req.Type, req.Input, req.WebhookURL)
	if err != nil {
		h.logger.Error("failed to create task", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create task"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":      t.ID,
		"type":    t.Type,
		"status":  t.Status,
		"message": "task created successfully",
	})
}

// GetTask returns a specific task by ID.
func (h *TaskHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")
	userObj := c.MustGet("user").(*models.User)

	t, err := h.taskService.GetTask(c.Request.Context(), parseUUID(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	// Ensure user owns this task
	if t.UserID != userObj.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, t)
}

// ListTasks returns a paginated list of tasks for the current user.
func (h *TaskHandler) ListTasks(c *gin.Context) {
	userObj := c.MustGet("user").(*models.User)

	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	tasks, total, err := h.taskService.ListTasks(c.Request.Context(), userObj.ID, status, limit, offset)
	if err != nil {
		h.logger.Error("failed to list tasks", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list tasks"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":   tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// CancelTask cancels a pending or running task.
func (h *TaskHandler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")
	userObj := c.MustGet("user").(*models.User)

	t, err := h.taskService.GetTask(c.Request.Context(), parseUUID(taskID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	if t.UserID != userObj.ID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	if t.Status != "pending" && t.Status != "running" {
		c.JSON(http.StatusConflict, gin.H{"error": "task cannot be cancelled in status: " + t.Status})
		return
	}

	if err := h.taskService.CancelTask(c.Request.Context(), parseUUID(taskID)); err != nil {
		h.logger.Error("failed to cancel task", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel task"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "task cancelled"})
}

// parseUUID is a helper to parse a UUID string, returning uuid.Nil on failure.
func parseUUID(s string) uuid.UUID {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}
