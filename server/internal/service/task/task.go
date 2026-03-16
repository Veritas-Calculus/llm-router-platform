// Package task provides asynchronous task management for long-running operations.
package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service manages async task lifecycle and webhook notifications.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new task service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// CreateTask creates a new async task.
func (s *Service) CreateTask(ctx context.Context, userID uuid.UUID, taskType, input, webhookURL string) (*models.AsyncTask, error) {
	// Validate webhook URL against SSRF before persisting
	if webhookURL != "" {
		if err := sanitize.ValidateWebhookURL(webhookURL, false); err != nil {
			return nil, fmt.Errorf("invalid webhook URL: %w", err)
		}
	}

	task := &models.AsyncTask{
		UserID:     userID,
		Type:       taskType,
		Status:     "pending",
		Input:      input,
		WebhookURL: webhookURL,
		Progress:   0,
	}

	if err := s.db.WithContext(ctx).Create(task).Error; err != nil {
		return nil, err
	}

	return task, nil
}

// GetTask retrieves a task by ID.
func (s *Service) GetTask(ctx context.Context, taskID uuid.UUID) (*models.AsyncTask, error) {
	var task models.AsyncTask
	if err := s.db.WithContext(ctx).First(&task, "id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ListTasks returns tasks for a user, optionally filtered by status.
func (s *Service) ListTasks(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]models.AsyncTask, int64, error) {
	var tasks []models.AsyncTask
	var total int64

	query := s.db.WithContext(ctx).Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&models.AsyncTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// UpdateProgress updates a task's progress percentage.
func (s *Service) UpdateProgress(ctx context.Context, taskID uuid.UUID, progress int) error {
	return s.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ?", taskID).
		Update("progress", progress).Error
}

// CompleteTask marks a task as completed with a result and fires webhook.
func (s *Service) CompleteTask(ctx context.Context, taskID uuid.UUID, result string) error {
	now := time.Now()
	err := s.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":       "completed",
			"result":       result,
			"progress":     100,
			"completed_at": &now,
		}).Error
	if err != nil {
		return err
	}

	// Fire webhook notification asynchronously
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		s.logger.Error("failed to get task for webhook", zap.Error(err))
		return nil // don't fail the completion
	}
	if task.WebhookURL != "" {
		go s.fireWebhook(task)
	}

	return nil
}

// FailTask marks a task as failed with an error message and fires webhook.
func (s *Service) FailTask(ctx context.Context, taskID uuid.UUID, errMsg string) error {
	now := time.Now()
	err := s.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":       "failed",
			"error":        errMsg,
			"completed_at": &now,
		}).Error
	if err != nil {
		return err
	}

	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		s.logger.Error("failed to get task for webhook", zap.Error(err))
		return nil
	}
	if task.WebhookURL != "" {
		go s.fireWebhook(task)
	}

	return nil
}

// CancelTask marks a task as cancelled.
func (s *Service) CancelTask(ctx context.Context, taskID uuid.UUID) error {
	now := time.Now()
	return s.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ? AND status IN ?", taskID, []string{"pending", "running"}).
		Updates(map[string]interface{}{
			"status":       "cancelled",
			"completed_at": &now,
		}).Error
}

// webhookPayload is the JSON payload sent to webhook URLs.
type webhookPayload struct {
	TaskID      string     `json:"task_id"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Progress    int        `json:"progress"`
	Result      string     `json:"result,omitempty"`
	Error       string     `json:"error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// fireWebhook sends a POST notification to the task's webhook URL with retry.
// Retries up to 3 times with exponential backoff (1s, 2s, 4s).
func (s *Service) fireWebhook(task *models.AsyncTask) {
	payload := webhookPayload{
		TaskID:      task.ID.String(),
		Type:        task.Type,
		Status:      task.Status,
		Progress:    task.Progress,
		Result:      task.Result,
		Error:       task.Error,
		CompletedAt: task.CompletedAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Error("failed to marshal webhook payload", zap.Error(err))
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s
			time.Sleep(backoff)
		}

		resp, err := client.Post(task.WebhookURL, "application/json", bytes.NewReader(body))
		if err != nil {
			s.logger.Warn("webhook delivery failed",
				zap.String("task_id", task.ID.String()),
				zap.String("webhook_url", task.WebhookURL),
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			continue
		}
		_ = resp.Body.Close()

		if resp.StatusCode < 400 {
			s.logger.Debug("webhook delivered",
				zap.String("task_id", task.ID.String()),
				zap.Int("attempt", attempt+1),
			)
			return // success
		}

		s.logger.Warn("webhook received non-success status",
			zap.String("task_id", task.ID.String()),
			zap.Int("status_code", resp.StatusCode),
			zap.Int("attempt", attempt+1),
		)
	}

	s.logger.Error("webhook delivery exhausted all retries",
		zap.String("task_id", task.ID.String()),
		zap.String("webhook_url", task.WebhookURL),
		zap.Int("max_retries", maxRetries),
	)
}
