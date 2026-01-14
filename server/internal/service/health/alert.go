// Package health provides health check functionality.
package health

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AlertNotifier handles alert notifications.
type AlertNotifier struct {
	alertRepo       *repository.AlertRepository
	alertConfigRepo *repository.AlertConfigRepository
	webhookClient   *http.Client
	logger          *zap.Logger
}

// NewAlertNotifier creates a new AlertNotifier.
func NewAlertNotifier(
	alertRepo *repository.AlertRepository,
	alertConfigRepo *repository.AlertConfigRepository,
	logger *zap.Logger,
) *AlertNotifier {
	return &AlertNotifier{
		alertRepo:       alertRepo,
		alertConfigRepo: alertConfigRepo,
		webhookClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

// GetAlerts retrieves alerts with pagination and optional status filter.
func (n *AlertNotifier) GetAlerts(ctx context.Context, status string, page, pageSize int) ([]models.Alert, int64, error) {
	offset := (page - 1) * pageSize
	return n.alertRepo.GetByStatus(ctx, status, offset, pageSize)
}

// AcknowledgeAlert marks an alert as acknowledged.
func (n *AlertNotifier) AcknowledgeAlert(ctx context.Context, alertID uuid.UUID) error {
	alert, err := n.alertRepo.GetByID(ctx, alertID)
	if err != nil {
		return err
	}

	now := time.Now()
	alert.Status = "acknowledged"
	alert.AcknowledgedAt = now

	return n.alertRepo.Update(ctx, alert)
}

// ResolveAlert marks an alert as resolved.
func (n *AlertNotifier) ResolveAlert(ctx context.Context, alertID uuid.UUID) error {
	alert, err := n.alertRepo.GetByID(ctx, alertID)
	if err != nil {
		return err
	}

	now := time.Now()
	alert.Status = "resolved"
	alert.ResolvedAt = now

	return n.alertRepo.Update(ctx, alert)
}

// GetAlertConfigByTarget returns alert config for a specific target.
func (n *AlertNotifier) GetAlertConfigByTarget(ctx context.Context, targetType string, targetID uuid.UUID) (*models.AlertConfig, error) {
	return n.alertConfigRepo.GetByTarget(ctx, targetType, targetID)
}

// UpdateAlertConfig updates or creates alert configuration.
func (n *AlertNotifier) UpdateAlertConfig(ctx context.Context, config *models.AlertConfig) error {
	existing, err := n.alertConfigRepo.GetByTarget(ctx, config.TargetType, config.TargetID)
	if err != nil {
		return n.alertConfigRepo.Create(ctx, config)
	}

	existing.IsEnabled = config.IsEnabled
	existing.FailureThreshold = config.FailureThreshold
	existing.WebhookURL = config.WebhookURL
	existing.Email = config.Email

	return n.alertConfigRepo.Update(ctx, existing)
}

// Notify sends an alert notification.
func (n *AlertNotifier) Notify(ctx context.Context, targetType string, targetID uuid.UUID, alertType, message string) error {
	alert := &models.Alert{
		TargetType: targetType,
		TargetID:   targetID,
		AlertType:  alertType,
		Message:    message,
		Status:     "active",
	}

	if err := n.alertRepo.Create(ctx, alert); err != nil {
		return err
	}

	config, err := n.alertConfigRepo.GetByTarget(ctx, targetType, targetID)
	if err != nil || !config.IsEnabled {
		return nil
	}

	if config.WebhookURL != "" {
		if err := n.sendWebhook(ctx, config.WebhookURL, alert); err != nil {
			n.logger.Error("failed to send webhook", zap.Error(err))
		}
	}

	return nil
}

// sendWebhook sends an alert via webhook.
func (n *AlertNotifier) sendWebhook(ctx context.Context, url string, alert *models.Alert) error {
	payload := map[string]interface{}{
		"target_type": alert.TargetType,
		"target_id":   alert.TargetID.String(),
		"alert_type":  alert.AlertType,
		"message":     alert.Message,
		"timestamp":   time.Now().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := n.webhookClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return errors.New("webhook request failed")
	}

	return nil
}

// Scheduler runs periodic health checks.
type Scheduler struct {
	healthService *Service
	interval      time.Duration
	stopCh        chan struct{}
	logger        *zap.Logger
}

// NewScheduler creates a new health check scheduler.
func NewScheduler(healthService *Service, interval time.Duration, logger *zap.Logger) *Scheduler {
	return &Scheduler{
		healthService: healthService,
		interval:      interval,
		stopCh:        make(chan struct{}),
		logger:        logger,
	}
}

// Start starts the health check scheduler.
func (s *Scheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.logger.Info("health check scheduler started", zap.Duration("interval", s.interval))

	for {
		select {
		case <-ticker.C:
			s.runHealthChecks(ctx)
		case <-s.stopCh:
			s.logger.Info("health check scheduler stopped")
			return
		case <-ctx.Done():
			s.logger.Info("health check scheduler context cancelled")
			return
		}
	}
}

// Stop stops the health check scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// runHealthChecks runs all health checks.
func (s *Scheduler) runHealthChecks(ctx context.Context) {
	s.logger.Debug("running scheduled health checks")

	apiKeyStatuses, err := s.healthService.GetAPIKeysHealth(ctx)
	if err != nil {
		s.logger.Error("failed to get API key statuses", zap.Error(err))
	} else {
		for _, status := range apiKeyStatuses {
			if _, err := s.healthService.CheckSingleAPIKey(ctx, status.ID); err != nil {
				s.logger.Error("failed to check API key health",
					zap.String("id", status.ID.String()),
					zap.Error(err))
			}
		}
	}

	proxyStatuses, err := s.healthService.GetProxiesHealth(ctx)
	if err != nil {
		s.logger.Error("failed to get proxy statuses", zap.Error(err))
	} else {
		for _, status := range proxyStatuses {
			if _, err := s.healthService.CheckSingleProxy(ctx, status.ID); err != nil {
				s.logger.Error("failed to check proxy health",
					zap.String("id", status.ID.String()),
					zap.Error(err))
			}
		}
	}
}
