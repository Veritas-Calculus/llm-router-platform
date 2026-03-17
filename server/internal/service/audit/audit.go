// Package audit provides security audit logging.
package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service records security-relevant events.
type Service struct {
	repo   repository.AuditLogRepo
	logger *zap.Logger
}

// NewService creates a new audit service.
func NewService(repo repository.AuditLogRepo, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
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

	if err := s.repo.Create(ctx, entry); err != nil {
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
	repoFilter := repository.AuditQueryFilter{
		ActorID: filter.ActorID,
		Action:  filter.Action,
		StartAt: filter.StartAt,
		EndAt:   filter.EndAt,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}
	return s.repo.Query(ctx, repoFilter)
}

// ExportCSV writes audit logs to a CSV writer with streaming pagination.
func (s *Service) ExportCSV(ctx context.Context, filter QueryFilter, w *csv.Writer) error {
	header := []string{"Timestamp", "Action", "Actor ID", "Target ID", "IP", "User Agent", "Detail"}
	if err := w.Write(header); err != nil {
		return err
	}

	repoFilter := repository.AuditQueryFilter{
		ActorID: filter.ActorID,
		Action:  filter.Action,
		StartAt: filter.StartAt,
		EndAt:   filter.EndAt,
	}

	const batchSize = 1000
	offset := 0
	for {
		logs, err := s.repo.QueryBatch(ctx, repoFilter, batchSize, offset)
		if err != nil {
			return err
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
	count, err := s.repo.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		s.logger.Info("purged old audit logs",
			zap.Int64("deleted", count),
			zap.Time("cutoff", cutoff),
		)
	}
	return count, nil
}

// Actions constants for audit logging.
const (
	ActionLogin             = "login"
	ActionLoginFailed       = "login_failed"
	ActionLogout            = "logout"
	ActionRegister          = "register"
	ActionPasswordChange    = "password_change"
	ActionRoleChange        = "role_change"
	ActionUserToggle        = "user_toggle"
	ActionAPIKeyCreate      = "apikey_create"
	ActionAPIKeyRevoke      = "apikey_revoke"
	ActionTokensInvalidated = "tokens_invalidated"
	ActionQuotaUpdate       = "quota_update"
)
