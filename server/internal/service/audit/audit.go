// Package audit provides security audit logging.
package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service records security-relevant events.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new audit service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Log records an audit event. This is fire-and-forget — never blocks the caller.
func (s *Service) Log(ctx context.Context, action string, actorID, targetID uuid.UUID, ip, userAgent string, detail map[string]interface{}) {
	detailJSON := ""
	if detail != nil {
		if b, err := json.Marshal(detail); err == nil {
			detailJSON = string(b)
		}
	}

	entry := &models.AuditLog{
		Action:    action,
		ActorID:   actorID,
		TargetID:  targetID,
		IP:        ip,
		UserAgent: userAgent,
		Detail:    detailJSON,
	}

	if err := s.db.WithContext(ctx).Create(entry).Error; err != nil {
		s.logger.Error("failed to write audit log",
			zap.String("action", action),
			zap.String("actor_id", actorID.String()),
			zap.Error(err),
		)
	}
}

// QueryFilter defines filters for audit log queries.
type QueryFilter struct {
	ActorID *uuid.UUID
	Action  string
	StartAt *time.Time
	EndAt   *time.Time
	Limit   int
	Offset  int
}

// Query retrieves audit logs with optional filtering and pagination.
func (s *Service) Query(ctx context.Context, filter QueryFilter) ([]models.AuditLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.AuditLog{})

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

// ExportCSV writes audit logs to a CSV writer with streaming pagination.
func (s *Service) ExportCSV(ctx context.Context, filter QueryFilter, w *csv.Writer) error {
	header := []string{"Timestamp", "Action", "Actor ID", "Target ID", "IP", "User Agent", "Detail"}
	if err := w.Write(header); err != nil {
		return err
	}

	const batchSize = 1000
	offset := 0
	for {
		query := s.db.WithContext(ctx).Model(&models.AuditLog{})
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
			return fmt.Errorf("failed to query audit logs (offset %d): %w", offset, err)
		}
		if len(logs) == 0 {
			break
		}

		for _, log := range logs {
			row := []string{
				log.CreatedAt.Format(time.RFC3339),
				log.Action,
				log.ActorID.String(),
				log.TargetID.String(),
				log.IP,
				log.UserAgent,
				log.Detail,
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		if len(logs) < batchSize {
			break
		}
		offset += batchSize
	}

	return nil
}

// PurgeOlderThan deletes audit logs older than the given retention period.
// Returns the number of deleted rows.
func (s *Service) PurgeOlderThan(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention)
	result := s.db.WithContext(ctx).
		Where("created_at < ?", cutoff).
		Delete(&models.AuditLog{})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		s.logger.Info("purged old audit logs",
			zap.Int64("deleted", result.RowsAffected),
			zap.Time("cutoff", cutoff),
		)
	}
	return result.RowsAffected, nil
}

// Actions constants for audit logging.
const (
	ActionLogin             = "login"
	ActionLoginFailed       = "login_failed"
	ActionRegister          = "register"
	ActionPasswordChange    = "password_change"
	ActionRoleChange        = "role_change"
	ActionUserToggle        = "user_toggle"
	ActionAPIKeyCreate      = "apikey_create"
	ActionAPIKeyRevoke      = "apikey_revoke"
	ActionTokensInvalidated = "tokens_invalidated"
	ActionQuotaUpdate       = "quota_update"
)
