package captcha

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

// fakeProvider is a providerSource that returns a constant. Used to swap
// the captcha backend in tests without bringing up the settings stack.
type fakeProvider struct{ value string }

func (f fakeProvider) CaptchaProvider(_ context.Context) string { return f.value }

func TestDevBackend_AcceptsBypassToken(t *testing.T) {
	s := New(zap.NewNop(), Config{DevBypass: "dev-ok"}, fakeProvider{value: string(ProviderDev)})
	if err := s.Verify(context.Background(), "dev-ok", "127.0.0.1"); err != nil {
		t.Fatalf("dev-ok should be accepted, got %v", err)
	}
}

func TestDevBackend_RejectsWrongToken(t *testing.T) {
	s := New(zap.NewNop(), Config{DevBypass: "dev-ok"}, fakeProvider{value: string(ProviderDev)})
	if err := s.Verify(context.Background(), "nope", "127.0.0.1"); err == nil {
		t.Fatal("expected rejection for wrong token")
	}
}

func TestDevBackend_RejectsEmpty(t *testing.T) {
	s := New(zap.NewNop(), Config{DevBypass: "dev-ok"}, fakeProvider{value: string(ProviderDev)})
	if err := s.Verify(context.Background(), "", "127.0.0.1"); err == nil {
		t.Fatal("empty token must be rejected even in dev mode")
	}
}

func TestDevBackend_DefaultBypass(t *testing.T) {
	// Empty DevBypass should fall back to "dev-ok".
	s := New(zap.NewNop(), Config{}, fakeProvider{value: string(ProviderDev)})
	if err := s.Verify(context.Background(), "dev-ok", ""); err != nil {
		t.Fatalf("default bypass should accept 'dev-ok', got %v", err)
	}
}

func TestDisabledBackend_AcceptsAnything(t *testing.T) {
	s := New(zap.NewNop(), Config{}, fakeProvider{value: string(ProviderDisabled)})
	if err := s.Verify(context.Background(), "", ""); err != nil {
		t.Fatalf("disabled backend should accept empty token, got %v", err)
	}
}

func TestUnknownProvider_FallsBackToDev(t *testing.T) {
	s := New(zap.NewNop(), Config{}, fakeProvider{value: "unknown"})
	if got := s.ProviderForCurrent(context.Background()); got != ProviderDev {
		t.Fatalf("expected fallback to dev, got %s", got)
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
		s := New(zap.NewNop(), Config{}, fakeProvider{value: string(c.p)})
		if got := s.EnabledForCurrent(context.Background()); got != c.want {
			t.Errorf("EnabledForCurrent() for %s = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestHotSwap_DevToDisabled(t *testing.T) {
	// Simulate an admin flipping the provider mid-flight: the same Service
	// instance should reject in dev mode (wrong token), then accept in
	// disabled mode without restart.
	src := &mutableProvider{value: string(ProviderDev)}
	s := New(zap.NewNop(), Config{DevBypass: "dev-ok"}, src)

	if err := s.Verify(context.Background(), "wrong", ""); err == nil {
		t.Fatal("dev mode must reject wrong token")
	}
	src.value = string(ProviderDisabled)
	if err := s.Verify(context.Background(), "wrong", ""); err != nil {
		t.Fatalf("disabled mode must accept any token, got %v", err)
	}
}

func TestTurnstileFailsClosedWithoutSecret(t *testing.T) {
	// Provider=turnstile but no SecretKey → must reject every token rather
	// than silently fall back to dev.
	s := New(zap.NewNop(), Config{}, fakeProvider{value: string(ProviderTurnstile)})
	if err := s.Verify(context.Background(), "any-token", ""); err == nil {
		t.Fatal("turnstile without secret key must fail closed")
	}
}

func TestHCaptchaFailsClosedWithoutSecret(t *testing.T) {
	s := New(zap.NewNop(), Config{}, fakeProvider{value: string(ProviderHCaptcha)})
	if err := s.Verify(context.Background(), "any-token", ""); err == nil {
		t.Fatal("hcaptcha without secret key must fail closed")
	}
}

// mutableProvider is a test helper that lets a test flip the active
// provider mid-run. Not concurrent-safe — tests synchronize externally.
type mutableProvider struct{ value string }

func (m *mutableProvider) CaptchaProvider(_ context.Context) string { return m.value }
