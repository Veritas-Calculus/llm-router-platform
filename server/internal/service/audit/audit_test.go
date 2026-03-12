package audit

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	logger := zap.NewNop()
	// Pass nil DB — the service should still be constructable
	svc := NewService(nil, logger)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestLogWithNilDB(t *testing.T) {
	logger := zap.NewNop()
	svc := NewService(nil, logger)

	// With nil DB, Log will panic due to gorm's nil pointer dereference.
	// This test verifies we detect this and that the caller should always
	// provide a non-nil DB. The service is designed to always have a DB.
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected Log with nil DB to panic, but it didn't")
		}
	}()
	svc.Log(context.Background(), ActionLogin, uuid.New(), uuid.Nil, "127.0.0.1", "test-agent", nil)
}

func TestActionConstants(t *testing.T) {
	// Ensure all action constants are non-empty and unique
	actions := []string{
		ActionLogin,
		ActionLoginFailed,
		ActionRegister,
		ActionPasswordChange,
		ActionRoleChange,
		ActionUserToggle,
		ActionAPIKeyCreate,
		ActionAPIKeyRevoke,
		ActionTokensInvalidated,
		ActionQuotaUpdate,
	}

	seen := make(map[string]bool)
	for _, a := range actions {
		if a == "" {
			t.Error("found empty action constant")
		}
		if seen[a] {
			t.Errorf("duplicate action constant: %s", a)
		}
		seen[a] = true
	}
}
