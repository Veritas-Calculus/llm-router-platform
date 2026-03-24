package config

import (
	"reflect"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// FeatureGates provides centralized, type-safe feature toggles for security-
// sensitive and operational capabilities. All gates are driven by environment
// variables with the prefix FG_ (e.g., FG_GRAPHQL_INTROSPECTION=true).
//
// Default philosophy:
//   - Security gates default to OFF (opt-in for safety)
//   - Auth gates default to OFF (require external config to be useful)
//   - Feature gates default to ON (core capabilities)
//   - Observability gates default to OFF (require external infra)
type FeatureGates struct {
	// ─── Security Gates (default: false) ───────────────────────────────
	// These expose debugging or schema information and must be explicitly
	// enabled in any environment.

	// GraphQLIntrospection enables __schema and __type introspection queries.
	GraphQLIntrospection bool `env:"FG_GRAPHQL_INTROSPECTION" gate:"security" desc:"GraphQL schema introspection (__schema, __type)"`

	// GraphQLPlayground enables the interactive GraphQL playground at GET /graphql.
	GraphQLPlayground bool `env:"FG_GRAPHQL_PLAYGROUND" gate:"security" desc:"GraphQL interactive playground UI"`

	// SwaggerDocs enables the Swagger API documentation at /swagger/*.
	SwaggerDocs bool `env:"FG_SWAGGER_DOCS" gate:"security" desc:"Swagger/OpenAPI documentation endpoint"`

	// PprofDebug enables pprof profiling endpoints at /debug/pprof/*.
	PprofDebug bool `env:"FG_PPROF_DEBUG" gate:"security" desc:"Go pprof profiling endpoints"`

	// AutoMigrate enables automatic database schema migration on startup.
	AutoMigrate bool `env:"FG_AUTO_MIGRATE" gate:"security" desc:"GORM AutoMigrate on startup (disable in production)"`

	// ─── Authentication Gates (default: false) ─────────────────────────

	// OAuth2Login enables GitHub/Google social login routes.
	OAuth2Login bool `env:"FG_OAUTH2_LOGIN" gate:"auth" desc:"GitHub/Google OAuth2 social login"`

	// SSOEnterprise enables SAML/OIDC enterprise SSO routes.
	SSOEnterprise bool `env:"FG_SSO_ENTERPRISE" gate:"auth" desc:"SAML/OIDC enterprise single sign-on"`

	// TwoFactorAuth enables TOTP-based two-factor authentication.
	TwoFactorAuth bool `env:"FG_2FA" gate:"auth" desc:"TOTP two-factor authentication"`

	// EmailVerification enables the email verification flow for new accounts.
	EmailVerification bool `env:"FG_EMAIL_VERIFICATION" gate:"auth" desc:"Email verification for new accounts"`

	// Turnstile enables Cloudflare Turnstile CAPTCHA protection.
	Turnstile bool `env:"FG_TURNSTILE" gate:"auth" desc:"Cloudflare Turnstile CAPTCHA"`

	// ─── Feature Gates (default: true) ─────────────────────────────────

	// SemanticCache enables exact-match and vector-similarity response caching.
	SemanticCache bool `env:"FG_SEMANTIC_CACHE" gate:"feature" desc:"Semantic response cache (exact + vector)"`

	// ConversationMemory enables server-side conversation history storage.
	ConversationMemory bool `env:"FG_CONVERSATION_MEMORY" gate:"feature" desc:"Server-side conversation memory"`

	// PromptSafety enables prompt injection detection (RuleEngine + Llama Guard).
	PromptSafety bool `env:"FG_PROMPT_SAFETY" gate:"feature" desc:"Prompt injection detection"`

	// MCPIntegration enables Model Context Protocol tool-calling support.
	MCPIntegration bool `env:"FG_MCP" gate:"feature" desc:"MCP tool integration"`

	// WebhookNotify enables webhook notification delivery.
	WebhookNotify bool `env:"FG_WEBHOOK" gate:"feature" desc:"Webhook event notifications"`

	// ─── Observability Gates (default: false) ──────────────────────────

	// MetricsUnauthenticated exposes /internal/metrics without JWT auth.
	MetricsUnauthenticated bool `env:"FG_METRICS_UNAUTH" gate:"observability" desc:"Unauthenticated Prometheus metrics endpoint"`

	// LangfuseTracing enables Langfuse LLM observability integration.
	LangfuseTracing bool `env:"FG_LANGFUSE" gate:"observability" desc:"Langfuse LLM tracing"`

	// SentryErrors enables Sentry error reporting.
	SentryErrors bool `env:"FG_SENTRY" gate:"observability" desc:"Sentry error reporting"`

	// OTelTracing enables OpenTelemetry distributed tracing.
	OTelTracing bool `env:"FG_OTEL" gate:"observability" desc:"OpenTelemetry distributed tracing"`
}

// GateInfo describes a single feature gate for API/admin consumption.
type GateInfo struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category"`
	Description string `json:"description"`
	EnvVar      string `json:"env_var"`
}

// loadFeatureGates reads all FG_ environment variables and returns a
// populated FeatureGates struct. Feature gates default to true; all
// others default to false.
func loadFeatureGates() FeatureGates {
	return FeatureGates{
		// Security — default OFF
		GraphQLIntrospection: viper.GetBool("FG_GRAPHQL_INTROSPECTION"),
		GraphQLPlayground:    viper.GetBool("FG_GRAPHQL_PLAYGROUND"),
		SwaggerDocs:          viper.GetBool("FG_SWAGGER_DOCS"),
		PprofDebug:           viper.GetBool("FG_PPROF_DEBUG"),
		AutoMigrate:          viper.GetBool("FG_AUTO_MIGRATE"),

		// Auth — default OFF
		OAuth2Login:       viper.GetBool("FG_OAUTH2_LOGIN"),
		SSOEnterprise:     viper.GetBool("FG_SSO_ENTERPRISE"),
		TwoFactorAuth:     viper.GetBool("FG_2FA"),
		EmailVerification: viper.GetBool("FG_EMAIL_VERIFICATION"),
		Turnstile:         viper.GetBool("FG_TURNSTILE"),

		// Feature — default ON
		SemanticCache:      viper.GetBool("FG_SEMANTIC_CACHE"),
		ConversationMemory: viper.GetBool("FG_CONVERSATION_MEMORY"),
		PromptSafety:       viper.GetBool("FG_PROMPT_SAFETY"),
		MCPIntegration:     viper.GetBool("FG_MCP"),
		WebhookNotify:      viper.GetBool("FG_WEBHOOK"),

		// Observability — default OFF
		MetricsUnauthenticated: viper.GetBool("FG_METRICS_UNAUTH"),
		LangfuseTracing:        viper.GetBool("FG_LANGFUSE"),
		SentryErrors:           viper.GetBool("FG_SENTRY"),
		OTelTracing:            viper.GetBool("FG_OTEL"),
	}
}

// setFeatureGateDefaults registers Viper defaults for all feature gates.
func setFeatureGateDefaults() {
	// Security — safe defaults (OFF)
	viper.SetDefault("FG_GRAPHQL_INTROSPECTION", false)
	viper.SetDefault("FG_GRAPHQL_PLAYGROUND", false)
	viper.SetDefault("FG_SWAGGER_DOCS", false)
	viper.SetDefault("FG_PPROF_DEBUG", false)
	viper.SetDefault("FG_AUTO_MIGRATE", false)

	// Auth — OFF
	viper.SetDefault("FG_OAUTH2_LOGIN", false)
	viper.SetDefault("FG_SSO_ENTERPRISE", false)
	viper.SetDefault("FG_2FA", false)
	viper.SetDefault("FG_EMAIL_VERIFICATION", false)
	viper.SetDefault("FG_TURNSTILE", false)

	// Feature — ON (core capabilities)
	viper.SetDefault("FG_SEMANTIC_CACHE", true)
	viper.SetDefault("FG_CONVERSATION_MEMORY", true)
	viper.SetDefault("FG_PROMPT_SAFETY", true)
	viper.SetDefault("FG_MCP", true)
	viper.SetDefault("FG_WEBHOOK", true)

	// Observability — OFF
	viper.SetDefault("FG_METRICS_UNAUTH", false)
	viper.SetDefault("FG_LANGFUSE", false)
	viper.SetDefault("FG_SENTRY", false)
	viper.SetDefault("FG_OTEL", false)
}

// ListGates returns all feature gates with their metadata for admin APIs.
func (fg *FeatureGates) ListGates() []GateInfo {
	var gates []GateInfo
	v := reflect.ValueOf(*fg)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		gates = append(gates, GateInfo{
			Name:        field.Name,
			Enabled:     v.Field(i).Bool(),
			Category:    field.Tag.Get("gate"),
			Description: field.Tag.Get("desc"),
			EnvVar:      field.Tag.Get("env"),
		})
	}
	return gates
}

// LogGates logs all feature gate states at startup for audit trail.
func (fg *FeatureGates) LogGates(logger *zap.Logger) {
	gates := fg.ListGates()

	enabled := 0
	for _, g := range gates {
		if g.Enabled {
			enabled++
		}
		logger.Info("feature gate",
			zap.String("gate", g.Name),
			zap.Bool("enabled", g.Enabled),
			zap.String("category", g.Category),
			zap.String("env", g.EnvVar),
		)
	}
	logger.Info("feature gates summary",
		zap.Int("total", len(gates)),
		zap.Int("enabled", enabled),
		zap.Int("disabled", len(gates)-enabled),
	)
}
