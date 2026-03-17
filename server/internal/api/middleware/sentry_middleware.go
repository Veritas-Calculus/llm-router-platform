// Package middleware provides HTTP middleware for the Gin router.
// This file implements Sentry error tracking middleware.
package middleware

import (
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
// authenticated user's information (set by the auth middleware earlier in the chain).
func SentryUserContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Only enrich if Sentry hub is active and user is set
		if hub := sentrygin.GetHubFromContext(c); hub != nil {
			userID, _ := c.Get("userID")
			email, _ := c.Get("email")

			if uid, ok := userID.(string); ok && uid != "" {
				hub.Scope().SetUser(sentry.User{
					ID:    uid,
					Email: emailStr(email),
				})
			}
		}
	}
}

func emailStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
