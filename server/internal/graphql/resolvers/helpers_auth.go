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
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const refreshTokenCookieName = "llm_router_refresh"

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
	gc.SetCookie(refreshTokenCookieName, refresh, int(ttl.Seconds()), "/", "", r.refreshCookieSecure(ctx), true)
}

func (r *mutationResolver) clearRefreshTokenCookie(ctx context.Context) {
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
		return
	}
	gc.SetSameSite(http.SameSiteLaxMode)
	gc.SetCookie(refreshTokenCookieName, "", -1, "/", "", r.refreshCookieSecure(ctx), true)
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

func (r *mutationResolver) refreshCookieSecure(ctx context.Context) bool {
	if r.Config().Server.Mode == "release" {
		return true
	}
	gc, err := directives.GinContextFromContext(ctx)
	if err != nil {
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

func (r *Resolver) verifyCaptcha(ctx context.Context, captchaToken *string) error {
	ip, _ := clientInfo(ctx)
	token := ""
	if captchaToken != nil {
		token = *captchaToken
	}
	return r.TurnstileSvc.Verify(ctx, token, ip)
}

func (r *Resolver) consumeInviteCode(ctx context.Context, code string) error {
	return r.DB().Transaction(func(tx *gorm.DB) error {
		var ic models.InviteCode
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("code = ?", code).First(&ic).Error; err != nil {
			return fmt.Errorf("invalid invite code")
		}
		if !ic.IsValid() {
			return fmt.Errorf("invite code is expired or exhausted")
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
