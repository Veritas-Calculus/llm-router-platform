package settings

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// fakeReader is a settingsReader that returns whatever the test plugs in.
// Atomic counter lets us assert how many times the cache reloaded.
type fakeReader struct {
	calls int32
	data  map[string]string
	err   error
}

func (f *fakeReader) GetAllSettingsDecrypted(_ context.Context) (map[string]string, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.err != nil {
		return nil, f.err
	}
	// Return a shallow copy so tests don't mutate the source.
	out := make(map[string]string, len(f.data))
	for k, v := range f.data {
		out[k] = v
	}
	return out, nil
}

func TestRegistry_DefaultsWhenNoRow(t *testing.T) {
	r := New(&fakeReader{}, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()
	if got := r.RegistrationMode(ctx); got != "open" {
		t.Errorf("RegistrationMode default = %q, want %q", got, "open")
	}
	if got := r.CookieSecureMode(ctx); got != "auto" {
		t.Errorf("CookieSecureMode default = %q, want %q", got, "auto")
	}
	if got := r.CaptchaProvider(ctx); got != "dev" {
		t.Errorf("CaptchaProvider default = %q, want %q", got, "dev")
	}
	if r.ProviderSyncAutoActivate(ctx) {
		t.Error("ProviderSyncAutoActivate default = true, want false")
	}
	if got := r.ProviderSyncBlocklistRegex(ctx); got != "" {
		t.Errorf("ProviderSyncBlocklistRegex default = %q, want empty", got)
	}
	if r.ProviderSyncBlocklistRe(ctx) != nil {
		t.Error("ProviderSyncBlocklistRe default should be nil (signal to use built-in)")
	}
}

func TestRegistry_ReadsSecurityCategory(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{"cookieSecureMode":"always","registrationMode":"invite"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()
	if got := r.RegistrationMode(ctx); got != "invite" {
		t.Errorf("RegistrationMode = %q, want %q", got, "invite")
	}
	if got := r.CookieSecureMode(ctx); got != "always" {
		t.Errorf("CookieSecureMode = %q, want %q", got, "always")
	}
}

func TestRegistry_ReadsDefaultsCategory(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"defaults": `{"providerSyncAutoActivate":true,"providerSyncBlocklistRegex":"(?i)test"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()
	if !r.ProviderSyncAutoActivate(ctx) {
		t.Error("ProviderSyncAutoActivate = false, want true")
	}
	if got := r.ProviderSyncBlocklistRegex(ctx); got != "(?i)test" {
		t.Errorf("ProviderSyncBlocklistRegex = %q, want %q", got, "(?i)test")
	}
	re := r.ProviderSyncBlocklistRe(ctx)
	if re == nil {
		t.Fatal("ProviderSyncBlocklistRe = nil, want compiled regex")
	}
	if !re.MatchString("This is a TEST string") {
		t.Error("compiled regex should match the test string case-insensitively")
	}
}

func TestRegistry_ReadsCaptchaCategory(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"captcha": `{"provider":"hcaptcha"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	if got := r.CaptchaProvider(context.Background()); got != "hcaptcha" {
		t.Errorf("CaptchaProvider = %q, want %q", got, "hcaptcha")
	}
}

func TestRegistry_RejectsInvalidValues(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{"cookieSecureMode":"yes please","registrationMode":"frozen"}`,
		"captcha":  `{"provider":"recaptcha"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()
	// Invalid values must NOT overwrite the defaults — the Registry
	// silently keeps the safe defaults so a typo can't break login.
	if got := r.RegistrationMode(ctx); got != "open" {
		t.Errorf("RegistrationMode = %q, want default %q", got, "open")
	}
	if got := r.CookieSecureMode(ctx); got != "auto" {
		t.Errorf("CookieSecureMode = %q, want default %q", got, "auto")
	}
	if got := r.CaptchaProvider(ctx); got != "dev" {
		t.Errorf("CaptchaProvider = %q, want default %q", got, "dev")
	}
}

func TestRegistry_MalformedJSONFallsBack(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{not json}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	if got := r.RegistrationMode(context.Background()); got != "open" {
		t.Errorf("RegistrationMode with malformed json = %q, want default", got)
	}
}

func TestRegistry_DBErrorFallsBack(t *testing.T) {
	reader := &fakeReader{err: errors.New("db down")}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	if got := r.RegistrationMode(context.Background()); got != "open" {
		t.Errorf("RegistrationMode with db error = %q, want default", got)
	}
}

func TestRegistry_CachesReads(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{"registrationMode":"closed"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()

	// Five consecutive reads should only trigger one DB call (the rest
	// are served from the cache).
	for i := 0; i < 5; i++ {
		r.RegistrationMode(ctx)
	}
	if got := atomic.LoadInt32(&reader.calls); got != 1 {
		t.Errorf("DB calls = %d, want 1 (caching not working)", got)
	}
}

func TestRegistry_InvalidateForcesRefresh(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{"registrationMode":"closed"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	ctx := context.Background()

	if got := r.RegistrationMode(ctx); got != "closed" {
		t.Fatalf("initial read = %q, want closed", got)
	}
	// Flip the DB row.
	reader.data["security"] = `{"registrationMode":"invite"}`
	// Without invalidation, the Registry still serves the cached value.
	if got := r.RegistrationMode(ctx); got != "closed" {
		t.Errorf("post-write read (no invalidate) = %q, want stale closed", got)
	}
	r.Invalidate(ctx, "security")
	if got := r.RegistrationMode(ctx); got != "invite" {
		t.Errorf("post-invalidate read = %q, want fresh invite", got)
	}
}

func TestRegistry_CacheTTLExpiry(t *testing.T) {
	reader := &fakeReader{data: map[string]string{
		"security": `{"registrationMode":"closed"}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	r.SetCacheTTL(10 * time.Millisecond)
	ctx := context.Background()

	r.RegistrationMode(ctx)
	if got := atomic.LoadInt32(&reader.calls); got != 1 {
		t.Fatalf("first read should hit DB once, got %d", got)
	}
	// Within TTL — cached.
	r.RegistrationMode(ctx)
	if got := atomic.LoadInt32(&reader.calls); got != 1 {
		t.Errorf("within-TTL read should be cached, got %d total calls", got)
	}
	// Past TTL — refetch.
	time.Sleep(15 * time.Millisecond)
	r.RegistrationMode(ctx)
	if got := atomic.LoadInt32(&reader.calls); got != 2 {
		t.Errorf("post-TTL read should refresh, got %d total calls (want 2)", got)
	}
}

func TestRegistry_NilReaderUsesDefaults(t *testing.T) {
	r := New(nil, BuiltinDefaults(), zap.NewNop())
	if got := r.RegistrationMode(context.Background()); got != "open" {
		t.Errorf("nil reader RegistrationMode = %q, want default", got)
	}
}

func TestRegistry_EmptyBlocklistKeepsBuiltinSignal(t *testing.T) {
	// Empty blocklist regex from DB means "use built-in default" — the
	// Registry should leave ProviderSyncBlocklistRe nil so the caller
	// (router) falls back to its own constant.
	reader := &fakeReader{data: map[string]string{
		"defaults": `{"providerSyncBlocklistRegex":""}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	if r.ProviderSyncBlocklistRe(context.Background()) != nil {
		t.Error("empty regex should leave compiled regex nil")
	}
}

func TestRegistry_MalformedRegexFallsBack(t *testing.T) {
	// A malformed regex should not crash — the Registry returns the raw
	// string but a nil compiled regex, signalling the caller to use the
	// built-in default.
	reader := &fakeReader{data: map[string]string{
		"defaults": `{"providerSyncBlocklistRegex":"["}`,
	}}
	r := New(reader, BuiltinDefaults(), zap.NewNop())
	if r.ProviderSyncBlocklistRe(context.Background()) != nil {
		t.Error("malformed regex should not produce a compiled regex")
	}
}
