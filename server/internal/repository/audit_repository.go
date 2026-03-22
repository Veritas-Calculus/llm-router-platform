package repository

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditLogRepository handles audit log data access.
type AuditLogRepository struct {
	db        *gorm.DB
	secretKey string // Used for HMAC-SHA256 signature chain
}

// NewAuditLogRepository creates a new audit log repository.
func NewAuditLogRepository(db *gorm.DB, secretKey string) *AuditLogRepository {
	return &AuditLogRepository{db: db, secretKey: secretKey}
}

// Create inserts a new audit log entry.
func (r *AuditLogRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. Find the last log to get its signature as our PreviousHash
		var lastLog models.AuditLog
		if err := tx.Order("created_at DESC, id DESC").First(&lastLog).Error; err == nil {
			entry.PreviousHash = lastLog.Signature
		} else if err == gorm.ErrRecordNotFound {
			entry.PreviousHash = "genesis"
		} else {
			return err
		}

		// 2. Compute HMAC signature logic
		// This ensures that the event is cryptographically tied to the chain.
		if entry.ID == uuid.Nil {
			entry.ID = uuid.New()
		}
		
		payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			entry.PreviousHash,
			entry.ID.String(),
			entry.Action,
			entry.ActorID.String(),
			entry.TargetID.String(),
			entry.Detail,
		)
		
		mac := hmac.New(sha256.New, []byte(r.secretKey))
		mac.Write([]byte(payload))
		entry.Signature = hex.EncodeToString(mac.Sum(nil))

		// 3. Insert the verified entry
		return tx.Create(entry).Error
	})
}

// AuditQueryFilter defines filters for audit log queries.
type AuditQueryFilter struct {
	ActorID *uuid.UUID
	Action  string
	StartAt *time.Time
	EndAt   *time.Time
	Limit   int
	Offset  int
}

// Query retrieves audit logs with optional filtering and pagination.
func (r *AuditLogRepository) Query(ctx context.Context, filter AuditQueryFilter) ([]models.AuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	if filter.ActorID != nil {
		query = query.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.StartAt != nil {
		query = query.Where("created_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("created_at <= ?", *filter.EndAt)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}

	var logs []models.AuditLog
	if err := query.Order("created_at DESC").
		Limit(filter.Limit).Offset(filter.Offset).
		Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// QueryBatch retrieves a batch of audit logs for streaming export.
func (r *AuditLogRepository) QueryBatch(ctx context.Context, filter AuditQueryFilter, batchSize, offset int) ([]models.AuditLog, error) {
	query := r.db.WithContext(ctx).Model(&models.AuditLog{})

	if filter.ActorID != nil {
		query = query.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.StartAt != nil {
		query = query.Where("created_at >= ?", *filter.StartAt)
	}
	if filter.EndAt != nil {
		query = query.Where("created_at <= ?", *filter.EndAt)
	}

	var logs []models.AuditLog
	if err := query.Order("created_at ASC").
		Limit(batchSize).Offset(offset).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to query audit logs (offset %d): %w", offset, err)
	}

	return logs, nil
}

// PurgeOlderThan deletes audit logs older than the given cutoff time.
// Returns the number of deleted rows.
func (r *AuditLogRepository) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&models.AuditLog{})
	return result.RowsAffected, result.Error
}
