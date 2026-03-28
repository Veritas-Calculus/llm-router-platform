// Package middleware provides HTTP middleware for the Gin router.
// This file implements Sentry error tracking middleware.
package middleware

import (
	"llm-router-platform/internal/models"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-gonic/gin"
)

// SentryMiddleware returns a Gin middleware that captures panics and errors to Sentry.
// It also attaches request context and user information to Sentry events.
//
// This middleware should be registered BEFORE gin.Recovery() so that Sentry captures
// the panic before Gin's recovery middleware swallows it.
func SentryMiddleware() gin.HandlerFunc {
	return sentrygin.New(sentrygin.Options{
		Repanic: true, // re-panic after capture so gin.Recovery() can log the stack trace
	})
}

// SentryUserContext returns middleware that enriches Sentry events with the
// authenticated user's information and project/org context for LLM API requests.
func SentryUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only enrich if Sentry hub is active
		hub := sentrygin.GetHubFromContext(c)
		if hub == nil {
			return
		}

		hub.ConfigureScope(func(scope *sentry.Scope) {
			// User context (set by JWT or API key auth middleware)
			userID, _ := c.Get("userID")
			email, _ := c.Get("email")

			if uid, ok := userID.(string); ok && uid != "" {
				scope.SetUser(sentry.User{
					ID:    uid,
					Email: emailStr(email),
				})
			}

			// Project context (set by API key auth middleware for LLM endpoints)
			if projectObj, exists := c.Get("project"); exists {
				if p, ok := projectObj.(*models.Project); ok {
					scope.SetTag("project_id", p.ID.String())
					scope.SetTag("org_id", p.OrgID.String())
				}
			}

			// API key context
			if apiKeyObj, exists := c.Get("api_key"); exists {
				if ak, ok := apiKeyObj.(*models.APIKey); ok {
					scope.SetTag("api_key_id", ak.ID.String())
					scope.SetTag("channel", ak.Channel)
				}
			}
		})
	}
}

func emailStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
