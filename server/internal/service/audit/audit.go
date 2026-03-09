// Package audit provides security audit logging.
package audit

import (
	"context"
	"encoding/json"

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
