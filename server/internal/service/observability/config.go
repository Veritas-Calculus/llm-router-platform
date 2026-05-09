package observability

import (
	"encoding/json"
	"fmt"
	"strings"

	"llm-router-platform/internal/config"
)

type langfuseIntegrationConfig struct {
	PublicKey string `json:"publicKey"`
	SecretKey string `json:"secretKey"`
	BaseURL   string `json:"baseUrl"`
	Host      string `json:"host"`
}

// IntegrationConfigOverridesRuntime returns true when a persisted integration row
// should override environment configuration. Empty default rows are created when
// the admin settings page is first opened, and those should not accidentally
// disable env-configured integrations.
func IntegrationConfigOverridesRuntime(enabled bool, raw []byte) bool {
	if enabled {
		return true
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return strings.TrimSpace(string(raw)) != ""
	}
	return len(obj) > 0
}

// LangfuseConfigFromIntegration merges a persisted IntegrationConfig payload
// into the process env config. Callers should only pass rows that are intended
// to override runtime config; disabled non-empty rows deliberately disable the
// Langfuse client.
func LangfuseConfigFromIntegration(base config.ObservabilityConfig, enabled bool, raw []byte) (config.ObservabilityConfig, error) {
	next := base
	next.LangfuseEnabled = enabled
	if !enabled {
		return next, nil
	}

	var cfg langfuseIntegrationConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return next, fmt.Errorf("invalid langfuse config JSON: %w", err)
	}

	next.LangfusePublicKey = strings.TrimSpace(cfg.PublicKey)
	next.LangfuseSecretKey = strings.TrimSpace(cfg.SecretKey)
	host := strings.TrimSpace(cfg.BaseURL)
	if host == "" {
		host = strings.TrimSpace(cfg.Host)
	}
	if host != "" {
		next.LangfuseHost = strings.TrimRight(host, "/")
	}

	if next.LangfusePublicKey == "" || next.LangfuseSecretKey == "" {
		return next, fmt.Errorf("langfuse publicKey and secretKey are required when enabled")
	}
	if next.LangfuseHost == "" {
		return next, fmt.Errorf("langfuse host is required when enabled")
	}

	return next, nil
}
