package middleware

import (
	"net"
	"net/http"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AdminIPWhitelist enforces IP filtering for the global management plane.
// Expects a comma-separated list of CIDRs or literal IPs.
func AdminIPWhitelist(whitelist string, logger *zap.Logger) gin.HandlerFunc {
	// If whitelist is empty, we fail open for local dev, but in prod this should be configured.
	if strings.TrimSpace(whitelist) == "" {
		return func(c *gin.Context) { c.Next() }
	}

	whitelistedSNs := ParseWhitelist(whitelist, logger)

	return func(c *gin.Context) {
		if !CheckIPAllowed(c.ClientIP(), whitelistedSNs, logger) {
			logger.Warn("Admin access blocked from non-whitelisted IP", zap.String("ip", sanitize.MaskIP(c.ClientIP())))
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden: IP not inside admin whitelist"})
			return
		}
		c.Next()
	}
}

// TenantAPIKeyWhitelist enforces IP filtering for LLM Proxy requests
// based on the Project's WhiteListedIps property.
// Must be used *after* the AuthMiddleware.APIKey module which populates "project" context.
func TenantAPIKeyWhitelist(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		val, exists := c.Get("project")
		if !exists {
			// If no project exists, fall through (maybe other auth handles it)
			c.Next()
			return
		}

		project, ok := val.(*models.Project)
		if !ok || project.WhiteListedIps == "" {
			// No whitelist defined for this tenant, fail open
			c.Next()
			return
		}

		whitelistedSNs := ParseWhitelist(project.WhiteListedIps, logger)
		if len(whitelistedSNs) > 0 {
			if !CheckIPAllowed(c.ClientIP(), whitelistedSNs, logger) {
				logger.Warn("API key access blocked from non-whitelisted IP", 
					zap.String("ip", sanitize.MaskIP(c.ClientIP())), 
					zap.String("project_id", project.ID.String()))
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden: IP not inside tenant whitelist"})
				return
			}
		}

		c.Next()
	}
}

// CheckIPAllowed verifies if a given string IP resides within any of the permitted subnets
func CheckIPAllowed(clientIP string, subnets []*net.IPNet, logger *zap.Logger) bool {
	parsedIP := net.ParseIP(clientIP)
	if parsedIP == nil {
		if logger != nil {
			logger.Warn("Failed to parse client IP", zap.String("ip", sanitize.MaskIP(clientIP)))
		}
		return false
	}

	for _, sn := range subnets {
		if sn.Contains(parsedIP) {
			return true
		}
	}
	return false
}

// ParseWhitelist converts a comma-separated string of IPs/CIDRs into a list of net.IPNet objects
func ParseWhitelist(whitelist string, logger *zap.Logger) []*net.IPNet {
	var snList []*net.IPNet
	parts := strings.Split(whitelist, ",")

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Try parsing as CIDR first
		_, sn, err := net.ParseCIDR(p)
		if err != nil {
			// Try parsing as single literal IP
			ip := net.ParseIP(p)
			if ip != nil {
				// Convert to CIDR notation dynamically 
				if ip.To4() != nil {
					p = p + "/32"
				} else {
					p = p + "/128"
				}
				_, sn, err = net.ParseCIDR(p)
			}
		}

		if err != nil {
			logger.Warn("Failed to parse whitelist entry, skipping", zap.String("entry", sanitize.LogValue(p)), zap.Error(err))
			continue
		}
		
		if sn != nil {
			snList = append(snList, sn)
		}
	}
	
	return snList
}
