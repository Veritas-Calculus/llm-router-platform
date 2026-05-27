package captcha

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestDevBackend_AcceptsBypassToken(t *testing.T) {
	s := New(zap.NewNop(), Config{Provider: ProviderDev, DevBypass: "dev-ok"})
	if err := s.Verify(context.Background(), "dev-ok", "127.0.0.1"); err != nil {
		t.Fatalf("dev-ok should be accepted, got %v", err)
	}
}

func TestDevBackend_RejectsWrongToken(t *testing.T) {
	s := New(zap.NewNop(), Config{Provider: ProviderDev, DevBypass: "dev-ok"})
	if err := s.Verify(context.Background(), "nope", "127.0.0.1"); err == nil {
		t.Fatal("expected rejection for wrong token")
	}
}

func TestDevBackend_RejectsEmpty(t *testing.T) {
	s := New(zap.NewNop(), Config{Provider: ProviderDev, DevBypass: "dev-ok"})
	if err := s.Verify(context.Background(), "", "127.0.0.1"); err == nil {
		t.Fatal("empty token must be rejected even in dev mode")
	}
}

func TestDevBackend_DefaultBypass(t *testing.T) {
	// Empty DevBypass should fall back to "dev-ok".
	s := New(zap.NewNop(), Config{Provider: ProviderDev})
	if err := s.Verify(context.Background(), "dev-ok", ""); err != nil {
		t.Fatalf("default bypass should accept 'dev-ok', got %v", err)
	}
}

func TestDisabledBackend_AcceptsAnything(t *testing.T) {
	s := New(zap.NewNop(), Config{Provider: ProviderDisabled})
	if err := s.Verify(context.Background(), "", ""); err != nil {
		t.Fatalf("disabled backend should accept empty token, got %v", err)
	}
}

func TestUnknownProvider_FallsBackToDev(t *testing.T) {
	s := New(zap.NewNop(), Config{Provider: "unknown"})
	if s.Provider() != ProviderDev {
		t.Fatalf("expected fallback to dev, got %s", s.Provider())
	}
}

func TestEnabled_Flag(t *testing.T) {
	cases := []struct {
		p    Provider
		want bool
	}{
		{ProviderDev, true},
		{ProviderTurnstile, true},
		{ProviderHCaptcha, true},
		{ProviderDisabled, false},
	}
	for _, c := range cases {
		s := New(zap.NewNop(), Config{Provider: c.p})
		if s.Enabled() != c.want {
			t.Errorf("Enabled() for %s = %v, want %v", c.p, s.Enabled(), c.want)
		}
	}
}
