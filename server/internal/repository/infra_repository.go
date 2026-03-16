// Package repository provides database access layer.
// This file contains health, proxy, usage, memory, and alert data access operations.
package repository

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProxyRepository handles proxy data access.
type ProxyRepository struct {
	db *gorm.DB
}

// NewProxyRepository creates a new proxy repository.
func NewProxyRepository(db *gorm.DB) *ProxyRepository {
	return &ProxyRepository{db: db}
}

// Create inserts a new proxy.
func (r *ProxyRepository) Create(ctx context.Context, proxy *models.Proxy) error {
	return r.db.WithContext(ctx).Create(proxy).Error
}

// GetByID retrieves a proxy by ID.
func (r *ProxyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Proxy, error) {
	var proxy models.Proxy
	if err := r.db.WithContext(ctx).First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

// GetActive retrieves all active proxies.
func (r *ProxyRepository) GetActive(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Where("is_active = ?", true).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// GetAll retrieves all proxies.
func (r *ProxyRepository) GetAll(ctx context.Context) ([]models.Proxy, error) {
	var proxies []models.Proxy
	if err := r.db.WithContext(ctx).Find(&proxies).Error; err != nil {
		return nil, err
	}
	return proxies, nil
}

// Update updates a proxy.
func (r *ProxyRepository) Update(ctx context.Context, proxy *models.Proxy) error {
	return r.db.WithContext(ctx).Save(proxy).Error
}

// Delete permanently removes a proxy from the database.
func (r *ProxyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Unscoped().Delete(&models.Proxy{}, "id = ?", id).Error
}

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

// HealthHistoryRepository handles health history data access.
type HealthHistoryRepository struct {
	db *gorm.DB
}

// NewHealthHistoryRepository creates a new health history repository.
func NewHealthHistoryRepository(db *gorm.DB) *HealthHistoryRepository {
	return &HealthHistoryRepository{db: db}
}

// Create inserts a new health history record.
func (r *HealthHistoryRepository) Create(ctx context.Context, history *models.HealthHistory) error {
	return r.db.WithContext(ctx).Create(history).Error
}

// GetByTarget retrieves health history for a target.
func (r *HealthHistoryRepository) GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID, limit int) ([]models.HealthHistory, error) {
	var histories []models.HealthHistory
	if err := r.db.WithContext(ctx).
		Where("target_type = ? AND target_id = ?", targetType, targetID).
		Order("checked_at DESC").
		Limit(limit).
		Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// GetRecent retrieves recent health history.
func (r *HealthHistoryRepository) GetRecent(ctx context.Context, targetType string, limit int) ([]models.HealthHistory, error) {
	var histories []models.HealthHistory
	query := r.db.WithContext(ctx).Order("checked_at DESC").Limit(limit)
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if err := query.Find(&histories).Error; err != nil {
		return nil, err
	}
	return histories, nil
}

// ConversationMemoryRepository handles conversation memory data access.
type ConversationMemoryRepository struct {
	db *gorm.DB
}

// NewConversationMemoryRepository creates a new conversation memory repository.
func NewConversationMemoryRepository(db *gorm.DB) *ConversationMemoryRepository {
	return &ConversationMemoryRepository{db: db}
}

// Create inserts a new conversation memory.
func (r *ConversationMemoryRepository) Create(ctx context.Context, memory *models.ConversationMemory) error {
	return r.db.WithContext(ctx).Create(memory).Error
}

// GetByConversation retrieves messages for a conversation.
func (r *ConversationMemoryRepository) GetByConversation(ctx context.Context, userID uuid.UUID, conversationID string) ([]models.ConversationMemory, error) {
	var memories []models.ConversationMemory
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("sequence ASC").
		Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

// DeleteByConversation permanently removes all messages in a conversation.
func (r *ConversationMemoryRepository) DeleteByConversation(ctx context.Context, userID uuid.UUID, conversationID string) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Delete(&models.ConversationMemory{}).Error
}

// DeleteOldestByConversation deletes the oldest N messages from a conversation.
func (r *ConversationMemoryRepository) DeleteOldestByConversation(ctx context.Context, userID uuid.UUID, conversationID string, count int) error {
	// Find the oldest N message IDs
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMemory{}).
		Where("user_id = ? AND conversation_id = ?", userID, conversationID).
		Order("sequence ASC").
		Limit(count).
		Pluck("id", &ids).Error; err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Unscoped().
		Where("id IN ?", ids).
		Delete(&models.ConversationMemory{}).Error
}

// ListConversationIDs returns all conversation IDs for a user.
func (r *ConversationMemoryRepository) ListConversationIDs(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMemory{}).
		Where("user_id = ?", userID).
		Distinct("conversation_id").
		Pluck("conversation_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
