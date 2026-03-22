package tracking

import (
	"context"
	"encoding/json"
	"fmt"
	"llm-router-platform/internal/models"

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
		var configs []models.IntegrationConfig
		if err := d.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
			d.logger.Error("failed to list enabled integrations", zap.Error(err))
			return
		}

		for _, cfg := range configs {
			switch cfg.Name {
			case "sentry":
				d.pushToSentry(log, cfg)
			case "loki":
				d.pushToLoki(log, cfg)
			case "langfuse":
				d.pushToLangfuse(log, cfg)
			default:
				d.logger.Warn("unknown tracking integration type", zap.String("name", cfg.Name))
			}
		}
	}(errLog)
}

func (d *Dispatcher) pushToSentry(log *models.ErrorLog, cfg models.IntegrationConfig) {
	var configData map[string]interface{}
	if err := json.Unmarshal(cfg.Config, &configData); err != nil {
		d.logger.Error("invalid sentry config", zap.Error(err))
		return
	}

	dsn, ok := configData["dsn"].(string)
	if !ok || dsn == "" {
		d.logger.Warn("sentry dsn missing in config")
		return
	}

	// Simulate pushing rich context directly to Sentry (for demo and log visibility)
	d.logger.Info(fmt.Sprintf("pushing to sentry (Trajectory: %s) -> DSN: %s", log.TrajectoryID, dsn),
		zap.String("trace_id", log.TraceID),
		zap.String("provider", log.Provider),
		zap.Int("status_code", log.StatusCode),
	)
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

	// Simulate pushing JSON formatted context strings to Loki analytical endpoints
	d.logger.Info(fmt.Sprintf("pushing to loki: %s", endpoint),
		zap.String("trajectory_id", log.TrajectoryID),
		zap.String("trace_id", log.TraceID),
		zap.String("model", log.Model),
	)
}

func (d *Dispatcher) pushToLangfuse(log *models.ErrorLog, cfg models.IntegrationConfig) {
	d.logger.Info("pushing to langfuse", zap.String("trajectory_id", log.TrajectoryID))
}
