// Package directives implements GraphQL schema directives.
package directives

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/gin-gonic/gin"

	"llm-router-platform/internal/graphql/model"
)

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
	}

	return next(ctx)
}

// RateLimit implements the @rateLimit directive.
// For now, this is a placeholder. The actual rate limiting is handled by
// the existing middleware on the /graphql route. In the future, this can
// implement per-field rate limiting using Redis.
func RateLimit(ctx context.Context, obj interface{}, next graphql.Resolver, max int, window string) (interface{}, error) {
	// TODO: Implement per-field rate limiting for auth mutations
	// For now, rely on route-level rate limiting
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
