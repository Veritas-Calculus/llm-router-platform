// Package handlers — oauth_exchange.go defines the shared HttpOnly-cookie
// based handoff used by both the OAuth2 and SSO callback handlers to deliver
// a freshly authenticated identity to the frontend WITHOUT placing the JWT in
// the redirect URL. The URL-token-in-callback antipattern leaks the access
// token into browser history, Referer headers, and edge proxy access logs.
//
// Flow:
//  1. Callback handler authenticates the user via the IdP.
//  2. Callback handler mints a short opaque code, stores
//     code → user_id in Redis with a 60-second TTL.
//  3. Callback handler sets an HttpOnly + Secure cookie named
//     OAuthExchangeCookieName containing the code, then 302s to the
//     frontend at /oauth/callback — no token in the URL.
//  4. The frontend calls the exchangeOAuthCode GraphQL mutation. That
//     resolver pulls the cookie, GETDELs the Redis entry, fetches the user,
//     and returns a normal AuthPayload.
package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	// OAuthExchangeCookieName is the cookie the callback handler writes and
	// the GraphQL resolver reads.
	OAuthExchangeCookieName = "oauth_exchange"
	// oauthExchangeTTL is the maximum time between the IdP callback and the
	// frontend's exchange mutation. 60s is enough for a slow CDN redirect
	// + slow client render; smaller is better because a leaked code is
	// useful for exactly this window.
	oauthExchangeTTL = 60 * time.Second
)

// OAuthExchangeRedisKey builds the Redis key for a given exchange code.
// Exported so the resolver in the graphql package can read it.
func OAuthExchangeRedisKey(code string) string {
	return "oauth:exchange:" + code
}

// MintOAuthExchangeCode generates a fresh code, stores user_id under it with
// the standard TTL, and sets the HttpOnly cookie on the response. Returns
// the cookie value (callers usually don't need it). On Redis failure the
// caller should fall back to the legacy URL-token redirect — but the same
// callers also have a hard dependency on Redis elsewhere (login limiter,
// rate limit, idempotency) so Redis-down is already a global outage.
func MintOAuthExchangeCode(ctx context.Context, redisClient *redis.Client, c *gin.Context, userID uuid.UUID) (string, error) {
	if redisClient == nil {
		return "", fmt.Errorf("redis unavailable")
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("entropy unavailable: %w", err)
	}
	code := base64.RawURLEncoding.EncodeToString(b)
	if err := redisClient.Set(ctx, OAuthExchangeRedisKey(code), userID.String(), oauthExchangeTTL).Err(); err != nil {
		return "", fmt.Errorf("store exchange code: %w", err)
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(OAuthExchangeCookieName, code, int(oauthExchangeTTL/time.Second), "/", "", true, true)
	return code, nil
}
