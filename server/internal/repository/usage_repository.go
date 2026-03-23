package repository

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UsageLogRepository handles usage log data access.
type UsageLogRepository struct {
	db *gorm.DB
}

// NewUsageLogRepository creates a new usage log repository.
func NewUsageLogRepository(db *gorm.DB) *UsageLogRepository {
	return &UsageLogRepository{db: db}
}

// Create inserts a new usage log.
func (r *UsageLogRepository) Create(ctx context.Context, log *models.UsageLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// GetByID returns a usage log by its ID.
func (r *UsageLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.UsageLog, error) {
	var log models.UsageLog
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&log).Error
	if err != nil {
		return nil, err
	}
	return &log, nil
}

// Update updates an existing usage log.
func (r *UsageLogRepository) Update(ctx context.Context, log *models.UsageLog) error {
	return r.db.WithContext(ctx).Save(log).Error
}

// GetByOrgOrProjectTimeRange retrieves usage logs for a specific org or project.
func (r *UsageLogRepository) GetByOrgOrProjectTimeRange(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, start, end time.Time) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select("usage_logs.*").
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Order("usage_logs.created_at DESC")

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByTimeRange retrieves all usage logs in time range (for system-wide stats).
func (r *UsageLogRepository) GetByTimeRange(ctx context.Context, start, end time.Time) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := r.db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByOrgOrProjectPaginated retrieves usage logs with LIMIT/OFFSET pagination.
func (r *UsageLogRepository) GetByOrgOrProjectPaginated(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, start, end time.Time, limit, offset int) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select("usage_logs.*").
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Order("usage_logs.created_at DESC").
		Limit(limit).Offset(offset)

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// GetByTimeRangePaginated retrieves all usage logs with LIMIT/OFFSET pagination.
func (r *UsageLogRepository) GetByTimeRangePaginated(ctx context.Context, start, end time.Time, limit, offset int) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := r.db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// ────────────────────────────────────────────────────────────────────────────
// SQL-level aggregation methods — avoid loading full rows into memory.
// ────────────────────────────────────────────────────────────────────────────

// UsageSummaryRow holds a single SQL-aggregated usage summary.
type UsageSummaryRow struct {
	TotalRequests int64   `json:"total_requests"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	AvgLatency    float64 `json:"avg_latency"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	MCPCallCount  int64   `json:"mcp_call_count"`
	MCPErrorCount int64   `json:"mcp_error_count"`
}

// AggregateByTimeRange returns SQL-aggregated usage for an org/project in a time range.
func (r *UsageLogRepository) AggregateByTimeRange(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, channel *string, start, end time.Time) (*UsageSummaryRow, error) {
	var row UsageSummaryRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`COUNT(usage_logs.id) AS total_requests,
				COALESCE(SUM(usage_logs.total_tokens), 0) AS total_tokens,
				COALESCE(SUM(usage_logs.cost), 0) AS total_cost,
				COALESCE(AVG(usage_logs.latency), 0) AS avg_latency,
				COALESCE(SUM(CASE WHEN usage_logs.status_code >= 200 AND usage_logs.status_code < 300 THEN 1 ELSE 0 END), 0) AS success_count,
				COALESCE(SUM(CASE WHEN usage_logs.status_code < 200 OR usage_logs.status_code >= 300 THEN 1 ELSE 0 END), 0) AS error_count,
				COALESCE(SUM(usage_logs.mcp_call_count), 0) AS mcp_call_count,
				COALESCE(SUM(usage_logs.mcp_error_count), 0) AS mcp_error_count`).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end)

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}
	if channel != nil && *channel != "" {
		query = query.Where("usage_logs.channel = ?", *channel)
	}

	if err := query.Scan(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// DailyUsageRow holds a single SQL-aggregated daily usage bucket.
type DailyUsageRow struct {
	Date     string  `json:"date"`
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	Cost     float64 `json:"cost"`
}

// AggregateDailyByTimeRange returns usage aggregated by day (SQL GROUP BY).
func (r *UsageLogRepository) AggregateDailyByTimeRange(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, channel *string, start, end time.Time) ([]DailyUsageRow, error) {
	var rows []DailyUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`TO_CHAR(usage_logs.created_at, 'YYYY-MM-DD') AS date,
				COUNT(usage_logs.id) AS requests,
				COALESCE(SUM(usage_logs.total_tokens), 0) AS tokens,
				COALESCE(SUM(usage_logs.cost), 0) AS cost`).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Group("TO_CHAR(usage_logs.created_at, 'YYYY-MM-DD')").
		Order("date")

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}
	if channel != nil && *channel != "" {
		query = query.Where("usage_logs.channel = ?", *channel)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ProviderUsageRow holds a single SQL-aggregated provider usage bucket.
type ProviderUsageRow struct {
	ProviderID   uuid.UUID `json:"provider_id"`
	ProviderName string    `json:"provider_name"`
	Requests     int64     `json:"requests"`
	Tokens       int64     `json:"tokens"`
	Cost         float64   `json:"cost"`
	SuccessRate  float64   `json:"success_rate"`
	AvgLatency   float64   `json:"avg_latency"`
}

// AggregateByProviderByTimeRange returns usage grouped by provider (SQL GROUP BY).
func (r *UsageLogRepository) AggregateByProviderByTimeRange(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, channel *string, start, end time.Time) ([]ProviderUsageRow, error) {
	var rows []ProviderUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Joins("LEFT JOIN providers ON usage_logs.provider_id = providers.id").
		Select(`usage_logs.provider_id,
				COALESCE(providers.name, '') AS provider_name,
				COUNT(usage_logs.id) AS requests,
				COALESCE(SUM(usage_logs.total_tokens), 0) AS tokens,
				COALESCE(SUM(usage_logs.cost), 0) AS cost,
				CASE WHEN COUNT(usage_logs.id) > 0
					THEN COALESCE(SUM(CASE WHEN usage_logs.status_code >= 200 AND usage_logs.status_code < 300 THEN 1 ELSE 0 END), 0) * 100.0 / COUNT(usage_logs.id)
					ELSE 0 END AS success_rate,
				COALESCE(AVG(usage_logs.latency), 0) AS avg_latency`).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Group("usage_logs.provider_id, providers.name")

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}
	if channel != nil && *channel != "" {
		query = query.Where("usage_logs.channel = ?", *channel)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ModelUsageRow holds a single SQL-aggregated model usage bucket.
type ModelUsageRow struct {
	ModelID      uuid.UUID `json:"model_id"`
	ModelName    string    `json:"model_name"`
	Requests     int64    `json:"requests"`
	InputTokens  int64    `json:"input_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	TotalTokens  int64    `json:"total_tokens"`
	Cost         float64  `json:"cost"`
}

// AggregateByModelByTimeRange returns usage grouped by model name (SQL GROUP BY).
func (r *UsageLogRepository) AggregateByModelByTimeRange(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, channel *string, start, end time.Time) ([]ModelUsageRow, error) {
	var rows []ModelUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`usage_logs.model_id, usage_logs.model_name,
				COUNT(usage_logs.id) AS requests,
				COALESCE(SUM(usage_logs.request_tokens), 0) AS input_tokens,
				COALESCE(SUM(usage_logs.response_tokens), 0) AS output_tokens,
				COALESCE(SUM(usage_logs.total_tokens), 0) AS total_tokens,
				COALESCE(SUM(usage_logs.cost), 0) AS cost`).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end).
		Where("usage_logs.model_name != ''").
		Group("usage_logs.model_id, usage_logs.model_name")

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}
	if channel != nil && *channel != "" {
		query = query.Where("usage_logs.channel = ?", *channel)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// CountByOrgOrProject counts total usage logs matching org/project in a time range (for pagination).
func (r *UsageLogRepository) CountByOrgOrProject(ctx context.Context, orgID *uuid.UUID, projectID *uuid.UUID, start, end time.Time) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Where("usage_logs.created_at >= ? AND usage_logs.created_at <= ?", start, end)

	if orgID != nil {
		query = query.Joins("JOIN projects ON usage_logs.project_id = projects.id").Where("projects.org_id = ?", *orgID)
	}
	if projectID != nil {
		query = query.Where("usage_logs.project_id = ?", *projectID)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetOrgIDByProjectID looks up the organization ID for a given project.
func (r *UsageLogRepository) GetOrgIDByProjectID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var project models.Project
	if err := r.db.WithContext(ctx).Select("org_id").Where("id = ?", projectID).First(&project).Error; err != nil {
		return uuid.Nil, err
	}
	return project.OrgID, nil
}
