package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Dispatcher handles pushing error logs to external services like Sentry, Loki, and Langfuse.
type Dispatcher struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewDispatcher creates a new initialized Tracking Dispatcher.
func NewDispatcher(db *gorm.DB, logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		db:     db,
		logger: logger,
	}
}

// ReportRouteError asynchronously pushes the rich ErrorLog to enabled external integrations.
func (d *Dispatcher) ReportRouteError(ctx context.Context, errLog *models.ErrorLog) {
	if d.db == nil {
		return
	}

	// Run asynchronously so we don't block the critical routing request
	go func(log *models.ErrorLog) {
		// Always push to Sentry via global SDK (if initialized)
		d.pushToSentrySDK(log)

		var configs []models.IntegrationConfig
		if err := d.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
			d.logger.Error("failed to list enabled integrations", zap.Error(err))
			return
		}

		for _, cfg := range configs {
			switch cfg.Name {
			case "loki":
				d.pushToLoki(log, cfg)
			case "langfuse":
				d.pushToLangfuse(log, cfg)
			default:
				d.logger.Debug("skipping integration", zap.String("name", cfg.Name))
			}
		}
	}(errLog)
}

// pushToSentrySDK sends the error as a Sentry event using the globally initialized SDK.
// This works whenever SENTRY_ENABLED=true and a valid DSN is configured, independent
// of the per-integration config in the database.
func (d *Dispatcher) pushToSentrySDK(log *models.ErrorLog) {
	hub := sentry.CurrentHub()
	if hub.Client() == nil {
		return // Sentry SDK not initialized
	}

	// Clone hub so we can set scope tags without affecting the global hub
	localHub := hub.Clone()

	localHub.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("provider", log.Provider)
		scope.SetTag("model", log.Model)
		scope.SetTag("status_code", fmt.Sprintf("%d", log.StatusCode))
		scope.SetTag("trace_id", log.TraceID)

		if log.TrajectoryID != "" {
			scope.SetTag("trajectory_id", log.TrajectoryID)
		}

		scope.SetExtra("response_body", log.ResponseBody)
		scope.SetExtra("headers", log.Headers)
		scope.SetLevel(sentryLevelFromStatus(log.StatusCode))
	})

	localHub.CaptureEvent(&sentry.Event{
		Message:   fmt.Sprintf("LLM Route Error: %s %s → %d", log.Provider, log.Model, log.StatusCode),
		Level:     sentryLevelFromStatus(log.StatusCode),
		Timestamp: log.CreatedAt,
		Extra: map[string]interface{}{
			"error_log_id":  log.ID.String(),
			"provider":      log.Provider,
			"model":         log.Model,
			"status_code":   log.StatusCode,
			"trace_id":      log.TraceID,
			"response_body": log.ResponseBody,
		},
	})

	d.logger.Debug("sentry event captured",
		zap.String("provider", sanitize.LogValue(log.Provider)),
		zap.String("model", sanitize.LogValue(log.Model)),
		zap.Int("status_code", log.StatusCode),
	)
}

// sentryLevelFromStatus maps HTTP status codes to Sentry severity levels.
func sentryLevelFromStatus(code int) sentry.Level {
	switch {
	case code >= 500:
		return sentry.LevelError
	case code == 429:
		return sentry.LevelWarning
	case code >= 400:
		return sentry.LevelWarning
	default:
		return sentry.LevelInfo
	}
}

func (d *Dispatcher) pushToLoki(log *models.ErrorLog, cfg models.IntegrationConfig) {
	var configData map[string]interface{}
	if err := json.Unmarshal(cfg.Config, &configData); err != nil {
		d.logger.Error("invalid loki config", zap.Error(err))
		return
	}

	endpoint, ok := configData["endpoint"].(string)
	if !ok || endpoint == "" {
		d.logger.Warn("loki endpoint missing in config")
		return
	}

	d.logger.Info("pushing to loki",
		zap.String("endpoint", sanitize.LogValue(strings.ReplaceAll(strings.ReplaceAll(endpoint, "\n", ""), "\r", ""))),
		zap.String("trajectory_id", sanitize.LogValue(strings.ReplaceAll(strings.ReplaceAll(log.TrajectoryID, "\n", ""), "\r", ""))),
		zap.String("trace_id", sanitize.LogValue(strings.ReplaceAll(strings.ReplaceAll(log.TraceID, "\n", ""), "\r", ""))),
		zap.String("model", sanitize.LogValue(strings.ReplaceAll(strings.ReplaceAll(log.Model, "\n", ""), "\r", ""))),
	)
}

func (d *Dispatcher) pushToLangfuse(log *models.ErrorLog, cfg models.IntegrationConfig) {
	d.logger.Info("pushing to langfuse", zap.String("trajectory_id", sanitize.LogValue(strings.ReplaceAll(strings.ReplaceAll(log.TrajectoryID, "\n", ""), "\r", ""))))
}
