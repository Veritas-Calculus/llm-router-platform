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

// GetByUserIDAndTimeRange retrieves usage logs for a user in time range.
func (r *UsageLogRepository) GetByUserIDAndTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, start, end).
		Order("created_at DESC").
		Find(&logs).Error; err != nil {
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

// GetByUserIDAndTimeRangePaginated retrieves usage logs for a user with LIMIT/OFFSET pagination.
func (r *UsageLogRepository) GetByUserIDAndTimeRangePaginated(ctx context.Context, userID uuid.UUID, start, end time.Time, limit, offset int) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND created_at >= ? AND created_at <= ?", userID, start, end).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&logs).Error; err != nil {
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
}

// AggregateByTimeRange returns SQL-aggregated usage for a user in a time range.
func (r *UsageLogRepository) AggregateByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) (*UsageSummaryRow, error) {
	var row UsageSummaryRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`COUNT(*) AS total_requests,
				COALESCE(SUM(total_tokens), 0) AS total_tokens,
				COALESCE(SUM(cost), 0) AS total_cost,
				COALESCE(AVG(latency), 0) AS avg_latency,
				COALESCE(SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END), 0) AS success_count,
				COALESCE(SUM(CASE WHEN status_code < 200 OR status_code >= 300 THEN 1 ELSE 0 END), 0) AS error_count`).
		Where("created_at >= ? AND created_at <= ?", start, end)

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
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
func (r *UsageLogRepository) AggregateDailyByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]DailyUsageRow, error) {
	var rows []DailyUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`TO_CHAR(created_at, 'YYYY-MM-DD') AS date,
				COUNT(*) AS requests,
				COALESCE(SUM(total_tokens), 0) AS tokens,
				COALESCE(SUM(cost), 0) AS cost`).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Group("TO_CHAR(created_at, 'YYYY-MM-DD')").
		Order("date")

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ProviderUsageRow holds a single SQL-aggregated provider usage bucket.
type ProviderUsageRow struct {
	ProviderID uuid.UUID `json:"provider_id"`
	Requests   int64     `json:"requests"`
	Tokens     int64     `json:"tokens"`
	Cost       float64   `json:"cost"`
}

// AggregateByProviderByTimeRange returns usage grouped by provider (SQL GROUP BY).
func (r *UsageLogRepository) AggregateByProviderByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]ProviderUsageRow, error) {
	var rows []ProviderUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`provider_id,
				COUNT(*) AS requests,
				COALESCE(SUM(total_tokens), 0) AS tokens,
				COALESCE(SUM(cost), 0) AS cost`).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Group("provider_id")

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
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
func (r *UsageLogRepository) AggregateByModelByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]ModelUsageRow, error) {
	var rows []ModelUsageRow
	query := r.db.WithContext(ctx).Model(&models.UsageLog{}).
		Select(`model_id, model_name,
				COUNT(*) AS requests,
				COALESCE(SUM(request_tokens), 0) AS input_tokens,
				COALESCE(SUM(response_tokens), 0) AS output_tokens,
				COALESCE(SUM(total_tokens), 0) AS total_tokens,
				COALESCE(SUM(cost), 0) AS cost`).
		Where("created_at >= ? AND created_at <= ?", start, end).
		Where("model_name != ''").
		Group("model_id, model_name")

	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
