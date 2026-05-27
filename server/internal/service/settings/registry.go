// Package settings exposes a small, typed read layer on top of the DB-
// backed system_configs table.
//
// Five runtime-policy knobs used to live in environment variables and were
// moved to the SystemSettings admin UI. The Registry is the single place
// that resolves a typed value with code-defined defaults applied — it
// caches the JSON blob for 5 minutes and exposes Invalidate so the
// updateSystemSettings GraphQL mutation can drop the cache on write.
//
// Layering: every domain that used to read cfg.<X>.<Y> for one of these
// fields now reads from the Registry instead. The Registry itself reads
// through config.Service (Resolver → Service → Repository → Model).
package settings

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"sync"
	"time"

	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
)

// settingsReader is the subset of *configService.Service the Registry
// needs. Defined as an interface so tests can inject a fake without
// pulling in the full config service.
type settingsReader interface {
	GetAllSettingsDecrypted(ctx context.Context) (map[string]string, error)
}

// defaultCacheTTL governs how stale the in-memory snapshot can get before
// we re-read from the DB. 5 minutes balances admin responsiveness against
// hammering Postgres on hot paths (captcha verify, every catalog sync,
// every Register call).
const defaultCacheTTL = 5 * time.Minute

// Defaults captures the built-in fallback values for every field the
// Registry exposes. Changing one of these here is the only way to change
// the "first boot, no DB row yet" behaviour.
type Defaults struct {
	RegistrationMode           string // "open" | "invite" | "closed"
	CookieSecureMode           string // "auto" | "always" | "never"
	CaptchaProvider            string // "dev" | "hcaptcha" | "turnstile" | "disabled"
	ProviderSyncAutoActivate   bool
	ProviderSyncBlocklistRegex string // empty → use built-in regex
}

// BuiltinDefaults are the safe, conservative defaults baked into the
// binary. These are NOT a global var so the test suite can clone and
// mutate without leaking state across tests.
func BuiltinDefaults() Defaults {
	return Defaults{
		RegistrationMode:           "open",
		CookieSecureMode:           "auto",
		CaptchaProvider:            "dev",
		ProviderSyncAutoActivate:   false,
		ProviderSyncBlocklistRegex: "",
	}
}

// Registry is the typed view over the DB-backed system settings. Safe
// for concurrent use.
type Registry struct {
	reader   settingsReader
	defaults Defaults
	logger   *zap.Logger
	ttl      time.Duration

	// Cached parsed values. Guarded by mu. fetchedAt zero means
	// "uninitialized — refetch on next read".
	mu        sync.RWMutex
	fetchedAt time.Time
	snapshot  snapshot

	// reload de-dupes concurrent cache misses so the DB only gets one
	// query per cache window even if hundreds of requests race for the
	// same expired snapshot.
	reload sync.Mutex
}

// snapshot captures the parsed-and-typed state of one DB read. We
// pre-compile the blocklist regex here so the hot path (every model name
// match during catalog sync) doesn't re-compile on every check.
type snapshot struct {
	registrationMode           string
	cookieSecureMode           string
	captchaProvider            string
	providerSyncAutoActivate   bool
	providerSyncBlocklistRegex string
	providerSyncBlocklistRe    *regexp.Regexp
}

// New returns a Registry backed by the supplied reader. If logger is nil
// it falls back to zap.NewNop so tests don't have to plumb a logger.
func New(reader settingsReader, defaults Defaults, logger *zap.Logger) *Registry {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Registry{
		reader:   reader,
		defaults: defaults,
		logger:   logger,
		ttl:      defaultCacheTTL,
	}
}

// SetCacheTTL overrides the cache duration. Intended for tests that need
// to verify cache expiry without sleeping for 5 minutes.
func (r *Registry) SetCacheTTL(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ttl = d
}

// Invalidate drops the cached snapshot so the next read goes back to the
// DB. The category argument is currently advisory — we always reload the
// full settings blob because the categories share one row each and
// re-reading is cheap. We keep the parameter so callers don't have to
// guess at the API and so we can add per-category caches later without
// breaking the call sites.
func (r *Registry) Invalidate(_ context.Context, category string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchedAt = time.Time{}
	if category != "" {
		r.logger.Info("settings registry cache invalidated",
			zap.String("category", sanitize.LogValue(category)))
	}
}

// RegistrationMode returns the active registration policy.
func (r *Registry) RegistrationMode(ctx context.Context) string {
	return r.snap(ctx).registrationMode
}

// CookieSecureMode returns the active cookie Secure-flag policy.
func (r *Registry) CookieSecureMode(ctx context.Context) string {
	return r.snap(ctx).cookieSecureMode
}

// CaptchaProvider returns the active captcha backend identifier.
func (r *Registry) CaptchaProvider(ctx context.Context) string {
	return r.snap(ctx).captchaProvider
}

// ProviderSyncAutoActivate reports whether newly-discovered models should
// be inserted active by default.
func (r *Registry) ProviderSyncAutoActivate(ctx context.Context) bool {
	return r.snap(ctx).providerSyncAutoActivate
}

// ProviderSyncBlocklistRegex returns the operator-supplied regex string.
// Empty means "use the built-in default" — callers should treat empty as
// a signal to apply their own fallback.
func (r *Registry) ProviderSyncBlocklistRegex(ctx context.Context) string {
	return r.snap(ctx).providerSyncBlocklistRegex
}

// ProviderSyncBlocklistRe returns the pre-compiled regex. May be nil if
// the operator-supplied pattern was malformed or empty; callers MUST
// fall back to their built-in default in that case.
func (r *Registry) ProviderSyncBlocklistRe(ctx context.Context) *regexp.Regexp {
	return r.snap(ctx).providerSyncBlocklistRe
}

// snap returns the cached snapshot, refreshing from DB if expired.
// Always non-zero — falls back to BuiltinDefaults on read errors so
// callers never have to nil-check.
func (r *Registry) snap(ctx context.Context) snapshot {
	r.mu.RLock()
	fresh := !r.fetchedAt.IsZero() && time.Since(r.fetchedAt) < r.ttl
	if fresh {
		s := r.snapshot
		r.mu.RUnlock()
		return s
	}
	r.mu.RUnlock()

	// Slow path: take the singleflight lock so concurrent misses share
	// one DB read.
	r.reload.Lock()
	defer r.reload.Unlock()

	// Re-check after acquiring reload — another goroutine may have
	// populated the cache while we were waiting.
	r.mu.RLock()
	if !r.fetchedAt.IsZero() && time.Since(r.fetchedAt) < r.ttl {
		s := r.snapshot
		r.mu.RUnlock()
		return s
	}
	r.mu.RUnlock()

	loaded := r.load(ctx)

	r.mu.Lock()
	r.snapshot = loaded
	r.fetchedAt = time.Now()
	r.mu.Unlock()

	return loaded
}

// load reads the raw settings JSON and parses out every field the
// Registry exposes. On any DB read or JSON-parse error we log a warning
// and return defaults — a misbehaving Postgres must not take down the
// whole platform.
func (r *Registry) load(ctx context.Context) snapshot {
	s := snapshot{
		registrationMode:         r.defaults.RegistrationMode,
		cookieSecureMode:         r.defaults.CookieSecureMode,
		captchaProvider:          r.defaults.CaptchaProvider,
		providerSyncAutoActivate: r.defaults.ProviderSyncAutoActivate,
	}
	s.providerSyncBlocklistRegex, s.providerSyncBlocklistRe = compileBlocklist(r.defaults.ProviderSyncBlocklistRegex)

	if r.reader == nil {
		return s
	}

	all, err := r.reader.GetAllSettingsDecrypted(ctx)
	if err != nil {
		r.logger.Warn("settings registry: read failed, using defaults",
			zap.Error(err))
		return s
	}

	// security category: cookieSecureMode + registrationMode
	if raw, ok := all["security"]; ok {
		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
			if v, ok := stringValue(parsed["cookieSecureMode"]); ok && validCookieMode(v) {
				s.cookieSecureMode = v
			}
			if v, ok := stringValue(parsed["registrationMode"]); ok && validRegistrationMode(v) {
				s.registrationMode = v
			}
		} else {
			r.logger.Warn("settings registry: security json parse failed",
				zap.Error(jsonErr))
		}
	}

	// defaults category: providerSyncAutoActivate + providerSyncBlocklistRegex
	if raw, ok := all["defaults"]; ok {
		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
			if v, ok := boolValue(parsed["providerSyncAutoActivate"]); ok {
				s.providerSyncAutoActivate = v
			}
			if v, ok := stringValue(parsed["providerSyncBlocklistRegex"]); ok {
				// Empty means "use built-in default" — keep what's already in the
				// snapshot (which was seeded from BuiltinDefaults above).
				if v != "" {
					raw, re := compileBlocklist(v)
					s.providerSyncBlocklistRegex = raw
					s.providerSyncBlocklistRe = re
				}
			}
		} else {
			r.logger.Warn("settings registry: defaults json parse failed",
				zap.Error(jsonErr))
		}
	}

	// captcha category: provider
	if raw, ok := all["captcha"]; ok {
		var parsed map[string]any
		if jsonErr := json.Unmarshal([]byte(raw), &parsed); jsonErr == nil {
			if v, ok := stringValue(parsed["provider"]); ok && validCaptchaProvider(v) {
				s.captchaProvider = v
			}
		} else {
			r.logger.Warn("settings registry: captcha json parse failed",
				zap.Error(jsonErr))
		}
	}

	return s
}

// compileBlocklist compiles a regex string. If the pattern is empty or
// invalid, returns (raw, nil) — callers fall back to their own default.
// We deliberately do NOT silently compile the built-in fallback here:
// the Registry is meant to surface "no override" via the nil return so
// the router package can keep using its own defaultBlocklistRegex
// without cross-package coupling.
func compileBlocklist(pattern string) (string, *regexp.Regexp) {
	trimmed := strings.TrimSpace(pattern)
	if trimmed == "" {
		return "", nil
	}
	re, err := regexp.Compile(trimmed)
	if err != nil {
		return trimmed, nil
	}
	return trimmed, re
}

// stringValue coerces a JSON-decoded value to a trimmed string. The
// "ok" return distinguishes "missing/wrong type" from "present empty".
func stringValue(v any) (string, bool) {
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(s), true
}

// boolValue coerces a JSON-decoded value to a bool. Accepts the literal
// true/false (the JSON-native form) only — string "true"/"false" would
// be a malformed write and we choose to reject it loudly via the
// default fallback.
func boolValue(v any) (bool, bool) {
	b, ok := v.(bool)
	if !ok {
		return false, false
	}
	return b, true
}

// validCookieMode rejects unknown values so a malformed admin write
// can't silently flip the cookie policy to something unexpected.
func validCookieMode(s string) bool {
	switch strings.ToLower(s) {
	case "auto", "always", "never":
		return true
	}
	return false
}

// validRegistrationMode mirrors the validator in config.go.
func validRegistrationMode(s string) bool {
	switch strings.ToLower(s) {
	case "open", "invite", "closed":
		return true
	}
	return false
}

// validCaptchaProvider matches captcha.Provider's exported constants.
func validCaptchaProvider(s string) bool {
	switch strings.ToLower(s) {
	case "dev", "hcaptcha", "turnstile", "disabled":
		return true
	}
	return false
}
