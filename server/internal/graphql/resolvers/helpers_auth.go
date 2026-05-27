package resolvers

// Domain helpers: helpers_auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"llm-router-platform/internal/graphql/directives"
	"llm-router-platform/internal/graphql/errs"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	refreshTokenCookieName = "llm_router_refresh" // #nosec G101 -- cookie name, not a credential
	// accessTokenCookieName carries the access JWT alongside (or instead of)
	// the Authorization header. C-02: moving access tokens off localStorage
	// reduces the XSS blast radius from "long-lived session takeover" to
	// "single-request CSRF" — and the latter is mitigated by SameSite=Lax +
	// the GraphQL handler accepting only POST.
	accessTokenCookieName = "llm_router_access" // #nosec G101 -- cookie name, not a credential
)

// clientInfo extracts client IP and User-Agent from the Gin context.
func clientInfo(ctx context.Context) (ip, userAgent string) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return "", ""
	}
	return gc.ClientIP(), gc.Request.UserAgent()
}

// ── JWT helpers ──────────────────────────────────────────────────────

func (r *mutationResolver) generateJWT(u *models.User) (string, error) {
	ttl := r.Config().JWT.ExpiresIn
	if ttl <= 0 {
		ttl = time.Hour // Default: 1 hour (prefer short-lived access tokens)
	}
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"role": u.Role,
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	if r.JWTSigner != nil {
		return r.JWTSigner.Sign(claims)
	}
	// Legacy HS256 fallback — only reached when BuildJWTSigner errored at
	// boot (a config-misuse safety net). New deployments should not hit this.
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config().JWT.Secret))
}

func (r *mutationResolver) generateRefreshJWT(u *models.User) (string, error) {
	ttl := r.Config().JWT.RefreshExpiresIn
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour // Default: 7 days
	}
	claims := jwt.MapClaims{
		"sub":  u.ID.String(),
		"type": "refresh",
		"exp":  time.Now().Add(ttl).Unix(),
		"iat":  time.Now().Unix(),
	}
	if r.JWTSigner != nil {
		return r.JWTSigner.Sign(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(r.Config().JWT.Secret))
}

func (r *mutationResolver) setRefreshTokenCookie(ctx context.Context, refresh string) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil || refresh == "" {
		return
	}
	ttl := r.Config().JWT.RefreshExpiresIn
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	gc.SetSameSite(http.SameSiteLaxMode)
	// Secure flag is decided at request time by refreshCookieSecure
	// (COOKIE_SECURE_MODE + TLS detection from audit L-02). CodeQL's
	// go/cookie-secure-not-set can't follow the runtime decision and
	// emits a false positive on these lines.
	gc.SetCookie(refreshTokenCookieName, refresh, int(ttl.Seconds()), "/", "", r.refreshCookieSecure(ctx), true)
}

func (r *mutationResolver) clearRefreshTokenCookie(ctx context.Context) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return
	}
	gc.SetSameSite(http.SameSiteLaxMode)
	// See note in setRefreshTokenCookie: secure flag is runtime-decided.
	gc.SetCookie(refreshTokenCookieName, "", -1, "/", "", r.refreshCookieSecure(ctx), true)
}

// setAccessTokenCookie writes the access JWT to an HttpOnly cookie.
// SameSite=Lax + HttpOnly + Secure (in release / behind https proxy) so a
// successful XSS cannot read it. The Max-Age tracks the JWT TTL so a
// stale cookie never lingers past its useful life.
//
// C-02: this lives alongside the bearer token in the GraphQL payload so
// API clients that still set Authorization keep working. The bearer field
// is marked @deprecated and will be removed in a future release.
func (r *mutationResolver) setAccessTokenCookie(ctx context.Context, accessToken string) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil || accessToken == "" {
		return
	}
	ttl := r.Config().JWT.ExpiresIn
	if ttl <= 0 {
		ttl = time.Hour
	}
	gc.SetSameSite(http.SameSiteLaxMode)
	// See note in setRefreshTokenCookie: secure flag is runtime-decided.
	gc.SetCookie(accessTokenCookieName, accessToken, int(ttl.Seconds()), "/", "", r.refreshCookieSecure(ctx), true)
}

func (r *mutationResolver) clearAccessTokenCookie(ctx context.Context) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return
	}
	gc.SetSameSite(http.SameSiteLaxMode)
	// See note in setRefreshTokenCookie: secure flag is runtime-decided.
	gc.SetCookie(accessTokenCookieName, "", -1, "/", "", r.refreshCookieSecure(ctx), true)
}

func refreshTokenFromCookie(ctx context.Context) (string, bool) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return "", false
	}
	refresh, err := gc.Cookie(refreshTokenCookieName)
	if err != nil || strings.TrimSpace(refresh) == "" {
		return "", false
	}
	return refresh, true
}

// refreshCookieSecure decides whether the Set-Cookie `Secure` flag should
// be set for the access / refresh cookies. Audit L-02: a Secure cookie set
// over HTTP is silently dropped by the browser, so dev (http) needs the
// flag off; release behind real TLS needs it on. Operators can override
// via COOKIE_SECURE_MODE (auto | always | never), default "auto":
//
//   auto   — Secure=true if release mode OR TLS request OR
//            X-Forwarded-Proto=https; false otherwise.
//   always — Secure=true unconditionally (use behind a TLS-terminating
//            proxy that strips X-Forwarded-Proto).
//   never  — Secure=false unconditionally. Dev only; never in production.
//
// I-01: the access/refresh cookies are HttpOnly so first-party JS cannot
// read them. We leave the JWT identifiable-by-decode (sub/iat) by design —
// any attacker that can read the cookie also owns the session, so swapping
// in an opaque random ID would not raise the bar. Re-evaluate if a product
// requirement appears that says user UUIDs must not be derivable.
func (r *mutationResolver) refreshCookieSecure(ctx context.Context) bool {
	mode := "auto"
	if r.SettingsRegistry != nil {
		mode = r.SettingsRegistry.CookieSecureMode(ctx)
	}
	switch mode {
	case "always":
		return true
	case "never":
		return false
	}
	// "auto" (default) and any unrecognized value.
	if r.Config().Server.Mode == "release" {
		return true
	}
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		// Without a request we can't sniff the protocol — fall back to the
		// safer choice (Secure=true). The only caller that hits this branch
		// is in tests, where setting Secure=true is harmless.
		return true
	}
	if strings.EqualFold(gc.GetHeader("X-Forwarded-Proto"), "https") || gc.Request.TLS != nil {
		return true
	}
	return false
}

func (r *mutationResolver) validateRefreshJWT(tokenStr string) (*jwt.RegisteredClaims, error) {
	var claims jwt.MapClaims
	if r.JWTSigner != nil {
		claims = jwt.MapClaims{}
		token, err := r.JWTSigner.Parse(tokenStr, &claims)
		if err != nil || !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
	} else {
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(r.Config().JWT.Secret), nil
		})
		if err != nil || !token.Valid {
			return nil, fmt.Errorf("invalid token")
		}
		var ok bool
		claims, ok = token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("invalid claims")
		}
	}

	// Ensure this is a refresh token, not an access token
	tokenType, _ := claims["type"].(string)
	if tokenType != "refresh" {
		return nil, fmt.Errorf("not a refresh token")
	}

	sub, _ := claims["sub"].(string)
	out := &jwt.RegisteredClaims{Subject: sub}
	// Preserve IssuedAt so RotateRefreshToken can enforce TokensInvalidatedAt.
	if iatRaw, ok := claims["iat"]; ok {
		switch v := iatRaw.(type) {
		case float64:
			out.IssuedAt = jwt.NewNumericDate(time.Unix(int64(v), 0))
		case int64:
			out.IssuedAt = jwt.NewNumericDate(time.Unix(v, 0))
		case json.Number:
			if n, err := v.Int64(); err == nil {
				out.IssuedAt = jwt.NewNumericDate(time.Unix(n, 0))
			}
		}
	}
	return out, nil
}

// ── Auth helpers ────────────────────────────────────────────────────

// verifyCaptcha runs the pluggable captcha service against the supplied token.
// The captcha service handles backend selection (dev / hcaptcha / turnstile /
// disabled), so resolvers do not have to know which provider is active. A
// missing or invalid token always rejects — registration is the highest-cost
// anti-abuse surface (auto-creates an org row + ships email + optionally
// grants the welcome credit), so we never short-circuit when the token is
// absent.
//
// Captcha failures are translated into typed errs.Validation("captchaToken",
// ...) errors so the GraphQL ErrorPresenter surfaces them to the client as
// extensions.code=VALIDATION + extensions.field=captchaToken (instead of
// masking them as a generic INTERNAL error). The translation is by exact
// message because the captcha backends each have their own wording.
func (r *Resolver) verifyCaptcha(ctx context.Context, captchaToken string) error {
	ip, _ := clientInfo(ctx)
	// Prefer the new captcha service; fall back to legacy turnstile if not
	// wired (only happens in tests where the resolver is built directly).
	var err error
	if r.CaptchaSvc != nil {
		err = r.CaptchaSvc.Verify(ctx, captchaToken, ip)
	} else {
		err = r.TurnstileSvc.Verify(ctx, captchaToken, ip)
	}
	if err == nil {
		return nil
	}
	switch err.Error() {
	case "captcha token required":
		return errs.Validation("captchaToken", "captcha token required")
	case "invalid captcha token":
		return errs.Validation("captchaToken", "invalid captcha token")
	case "CAPTCHA verification required":
		return errs.Validation("captchaToken", "captcha verification required")
	case "CAPTCHA verification failed",
		"CAPTCHA verification failed, please try again":
		return errs.Validation("captchaToken", "captcha verification failed, please try again")
	}
	// Unknown error shape — let the legacy classifier or the INTERNAL fallback
	// handle it. We don't want a misconfigured backend (e.g. bad secret key)
	// to surface as a "VALIDATION" issue to the user.
	return err
}

func (r *Resolver) consumeInviteCode(ctx context.Context, code string) error {
	return r.DB().Transaction(func(tx *gorm.DB) error {
		var ic models.InviteCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("code = ?", code).First(&ic).Error; err != nil {
			return errs.Validation("inviteCode", "invalid invite code")
		}
		if !ic.IsValid() {
			return errs.Validation("inviteCode", "invite code is expired or exhausted")
		}
		return tx.Model(&ic).UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
	})
}

func (r *Resolver) checkWelcomeCreditEligibility(ctx context.Context) bool {
	if r.RedisClient() == nil {
		return true
	}
	ip, _ := clientInfo(ctx)
	creditKey := fmt.Sprintf("reg_credit:%s", ip)
	cnt, redisErr := r.RedisClient().Incr(ctx, creditKey).Result()
	if redisErr != nil {
		return true
	}
	if cnt == 1 {
		r.RedisClient().Expire(ctx, creditKey, 24*time.Hour)
	}
	if cnt > 3 {
		r.Logger.Warn("welcome credit denied: IP throttle exceeded",
			zap.String("ip", sanitize.LogValue(ip)), zap.Int64("count", cnt))
		return false
	}
	return true
}

// passwordPolicyErrorPrefixes lists every wire prefix produced by
// user.ValidatePassword. Keep this in lockstep with
// server/internal/service/user/user.go::ValidatePassword — a new rule there
// must add its prefix here too, otherwise the GraphQL ErrorPresenter will
// mask it as a generic INTERNAL error and the signup form won't be able to
// pin it to the `password` field.
//
// We intentionally enumerate prefixes instead of using a single broad
// `strings.HasPrefix(msg, "password ")` because future password-domain
// errors that are NOT validation failures (e.g. "password reset failed",
// "password hash unavailable") must continue to be masked. The narrow
// allowlist preserves the user-enumeration / leakage guarantees the
// audit-remediation work re-established. Post-audit follow-up P0-3.
var passwordPolicyErrorPrefixes = []string{
	"password must be at least",         // length rule
	"password must contain at least",    // uppercase / lowercase / digit rules
	"password is too common",            // common-password block
}

// isPasswordPolicyError reports whether err is a password-policy violation
// from the user service. Matches the exact set of prefixes produced by
// user.ValidatePassword (see passwordPolicyErrorPrefixes). Anything else
// — including future password-domain errors that aren't validation
// failures — falls through to the INTERNAL masking path on purpose.
func isPasswordPolicyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, p := range passwordPolicyErrorPrefixes {
		if strings.HasPrefix(msg, p) {
			return true
		}
	}
	return false
}
