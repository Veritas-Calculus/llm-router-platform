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
