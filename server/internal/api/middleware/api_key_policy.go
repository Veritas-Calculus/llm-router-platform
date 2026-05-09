package middleware

import (
	"net/http"
	"strings"

	"llm-router-platform/internal/models"

	"github.com/gin-gonic/gin"
)

// APIKeyScope enforces coarse per-key endpoint scopes such as chat, embeddings, images, and audio.
func APIKeyScope(scope string) gin.HandlerFunc {
	scope = strings.ToLower(strings.TrimSpace(scope))
	return func(c *gin.Context) {
		if scope == "" {
			c.Next()
			return
		}

		keyVal, ok := c.Get("api_key")
		if !ok {
			c.Next()
			return
		}
		apiKey, ok := keyVal.(*models.APIKey)
		if !ok || apiKey == nil {
			c.Next()
			return
		}
		if apiKey.AllowsScope(scope) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": "API key is not allowed to access " + scope,
				"type":    "forbidden",
				"code":    "api_key_scope_forbidden",
			},
		})
	}
}
