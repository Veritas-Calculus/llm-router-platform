// Package directives implements GraphQL schema directives.
package directives

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"

	"llm-router-platform/internal/api/middleware"
	"llm-router-platform/internal/graphql/model"
)

// RedisClient is the minimal Redis interface needed for rate limiting.
type RedisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

// contextKey is used to store user info in context.
type contextKey string

const (
	// GinContextKey is used to extract *gin.Context from context.
	GinContextKey contextKey = "GinContext"
)

// GinContextFromContext extracts *gin.Context from the Go context.
func GinContextFromContext(ctx context.Context) (*gin.Context, error) {
	gc, ok := ctx.Value(GinContextKey).(*gin.Context)
	if !ok {
		return nil, fmt.Errorf("could not retrieve gin.Context")
	}
	return gc, nil
}

// Auth implements the @auth directive.
// It validates that the request has a valid JWT and optionally checks the role.
// The JWT middleware sets "user_id", "email", and "role" in the Gin context.
func Auth(ctx context.Context, obj interface{}, next graphql.Resolver, role *model.Role) (interface{}, error) {
	gc, err := GinContextFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unauthorized")
	}

	// Check if user_id is set (JWT middleware has run)
	userID, exists := gc.Get("user_id")
	if !exists || userID == "" {
		return nil, fmt.Errorf("unauthorized: authentication required")
	}

	// If a role is required, check it
	if role != nil && *role == model.RoleAdmin {
		userRole, _ := gc.Get("role")
		if userRole != "admin" {
			return nil, fmt.Errorf("forbidden: admin access required")
		}

		// Enforce Admin IP Whitelist
		if whitelistStr, exists := gc.Get("admin_ip_whitelist"); exists {
			whitelist := whitelistStr.(string)
			if whitelist != "" {
				whitelistedSNs := middleware.ParseWhitelist(whitelist, nil)
				if len(whitelistedSNs) > 0 {
					if !middleware.CheckIPAllowed(gc.ClientIP(), whitelistedSNs, nil) {
						return nil, fmt.Errorf("forbidden: IP not inside admin whitelist")
					}
				}
			}
		}
	}

	return next(ctx)
}

// securityCriticalFields lists GraphQL field names where rate limiting MUST NOT
// fail open. When Redis is unavailable these fields are rejected outright to
// prevent brute-force and bulk-registration attacks.
var securityCriticalFields = map[string]bool{
	"register":       true,
	"login":          true,
	"forgotPassword": true,
	"resetPassword":  true,
}

// RateLimit implements the @rateLimit directive with per-field sliding window
// rate limiting using Redis.
//
// Security-critical fields (register, login, forgotPassword, resetPassword) use
// fail-closed semantics: if Redis is unreachable the request is rejected.
// All other fields fail open so that transient Redis issues do not block normal
// usage — route-level middleware still provides defense-in-depth.
func RateLimit(ctx context.Context, obj interface{}, next graphql.Resolver, max int, window string) (interface{}, error) {
	gc, err := GinContextFromContext(ctx)
	if err != nil {
		return next(ctx) // can't rate-limit without gin context
	}

	// Resolve field name early so we can decide fail-open vs fail-closed.
	fieldCtx := graphql.GetFieldContext(ctx)
	fieldName := "unknown"
	if fieldCtx != nil {
		fieldName = fieldCtx.Field.Name
	}
	critical := securityCriticalFields[fieldName]

	// Get Redis client from gin context (set in routes.go)
	redisVal, exists := gc.Get("redis")
	if !exists {
		if critical {
			return nil, fmt.Errorf("service temporarily unavailable, please try again later")
		}
		return next(ctx) // non-critical: fall through
	}
	rdb, ok := redisVal.(RedisClient)
	if !ok {
		if critical {
			return nil, fmt.Errorf("service temporarily unavailable, please try again later")
		}
		return next(ctx)
	}

	// Parse window duration
	dur, err := time.ParseDuration(window)
	if err != nil {
		return next(ctx) // invalid window config, fall through
	}

	// Build rate limit key: gql_rl:{fieldName}:{clientIP}
	clientIP := gc.ClientIP()
	key := fmt.Sprintf("gql_rl:%s:%s", fieldName, clientIP)

	// Sliding window: increment and check
	count, redisErr := rdb.Incr(ctx, key).Result()
	if redisErr != nil {
		if critical {
			return nil, fmt.Errorf("service temporarily unavailable, please try again later")
		}
		return next(ctx) // non-critical: fail open
	}

	// Set TTL on first increment
	if count == 1 {
		rdb.Expire(ctx, key, dur)
	}

	if count > int64(max) {
		return nil, fmt.Errorf("rate limit exceeded: try again later")
	}

	return next(ctx)
}

// UserIDFromContext extracts the authenticated user's ID from the context.
func UserIDFromContext(ctx context.Context) (string, error) {
	gc, err := GinContextFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("unauthorized")
	}
	userID, exists := gc.Get("user_id")
	if !exists {
		return "", fmt.Errorf("unauthorized: no user in context")
	}
	idStr, ok := userID.(string)
	if !ok {
		return "", fmt.Errorf("unauthorized: invalid user ID")
	}
	return idStr, nil
}

// UserRoleFromContext extracts the authenticated user's role from the context.
func UserRoleFromContext(ctx context.Context) (string, error) {
	gc, err := GinContextFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("unauthorized")
	}
	role, exists := gc.Get("role")
	if !exists {
		return "", fmt.Errorf("unauthorized: no role in context")
	}
	roleStr, ok := role.(string)
	if !ok {
		return "", fmt.Errorf("unauthorized: invalid role")
	}
	return roleStr, nil
}
