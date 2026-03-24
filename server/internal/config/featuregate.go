package config

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// FeatureGates provides centralized, type-safe feature toggles for security-
// sensitive and operational capabilities.
//
// Gate values are resolved in priority order:
//
//	Environment variable FG_* > DB (system_configs) > Code default
//
// Default philosophy:
//   - Security gates default to OFF (opt-in for safety)
//   - Auth gates default to OFF (require external config to be useful)
//   - Feature gates default to ON (core capabilities)
//   - Observability gates default to OFF (require external infra)
type FeatureGates struct {
	// ─── Security Gates (default: false) ───────────────────────────────
	GraphQLIntrospection bool `env:"FG_GRAPHQL_INTROSPECTION" gate:"security" desc:"GraphQL schema introspection (__schema, __type)"`
	GraphQLPlayground    bool `env:"FG_GRAPHQL_PLAYGROUND"    gate:"security" desc:"GraphQL interactive playground UI"`
	SwaggerDocs          bool `env:"FG_SWAGGER_DOCS"          gate:"security" desc:"Swagger/OpenAPI documentation endpoint"`
	PprofDebug           bool `env:"FG_PPROF_DEBUG"           gate:"security" desc:"Go pprof profiling endpoints"`
	AutoMigrate          bool `env:"FG_AUTO_MIGRATE"          gate:"security" desc:"GORM AutoMigrate on startup (disable in production)"`

	// ─── Authentication Gates (default: false) ─────────────────────────
	OAuth2Login       bool `env:"FG_OAUTH2_LOGIN"        gate:"auth" desc:"GitHub/Google OAuth2 social login"`
	SSOEnterprise     bool `env:"FG_SSO_ENTERPRISE"      gate:"auth" desc:"SAML/OIDC enterprise single sign-on"`
	TwoFactorAuth     bool `env:"FG_2FA"                 gate:"auth" desc:"TOTP two-factor authentication"`
	EmailVerification bool `env:"FG_EMAIL_VERIFICATION"  gate:"auth" desc:"Email verification for new accounts"`
	Turnstile         bool `env:"FG_TURNSTILE"           gate:"auth" desc:"Cloudflare Turnstile CAPTCHA"`

	// ─── Feature Gates (default: true) ─────────────────────────────────
	SemanticCache      bool `env:"FG_SEMANTIC_CACHE"       gate:"feature" desc:"Semantic response cache (exact + vector)"`
	ConversationMemory bool `env:"FG_CONVERSATION_MEMORY"  gate:"feature" desc:"Server-side conversation memory"`
	PromptSafety       bool `env:"FG_PROMPT_SAFETY"        gate:"feature" desc:"Prompt injection detection"`
	MCPIntegration     bool `env:"FG_MCP"                  gate:"feature" desc:"MCP tool integration"`
	WebhookNotify      bool `env:"FG_WEBHOOK"              gate:"feature" desc:"Webhook event notifications"`

	// ─── Observability Gates (default: false) ──────────────────────────
	MetricsUnauthenticated bool `env:"FG_METRICS_UNAUTH" gate:"observability" desc:"Unauthenticated Prometheus metrics endpoint"`
	LangfuseTracing        bool `env:"FG_LANGFUSE"       gate:"observability" desc:"Langfuse LLM tracing"`
	SentryErrors           bool `env:"FG_SENTRY"         gate:"observability" desc:"Sentry error reporting"`
	OTelTracing            bool `env:"FG_OTEL"           gate:"observability" desc:"OpenTelemetry distributed tracing"`

	// ─── Runtime state (not serialized) ────────────────────────────────
	mu       sync.RWMutex       // protects gates map
	gates    map[string]bool    // canonical runtime state
	dbValues map[string]bool    // values loaded from DB
	meta     map[string]gateMeta // field name -> metadata
}

// gateMeta holds immutable metadata for a single gate, parsed once from struct tags.
type gateMeta struct {
	EnvVar      string
	Category    string
	Description string
}

// GateInfo describes a single feature gate for API/admin consumption.
type GateInfo struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category"`
	Description string `json:"description"`
	EnvVar      string `json:"env_var"`
	Source      string `json:"source"` // "default", "database", "env_override"
	Locked      bool   `json:"locked"` // true if env override prevents DB modification
}

// loadFeatureGates reads all FG_ environment variables and returns a
// pointer to a populated FeatureGates struct with initialized runtime maps.
func loadFeatureGates() *FeatureGates {
	fg := &FeatureGates{
		// Security
		GraphQLIntrospection: viper.GetBool("FG_GRAPHQL_INTROSPECTION"),
		GraphQLPlayground:    viper.GetBool("FG_GRAPHQL_PLAYGROUND"),
		SwaggerDocs:          viper.GetBool("FG_SWAGGER_DOCS"),
		PprofDebug:           viper.GetBool("FG_PPROF_DEBUG"),
		AutoMigrate:          viper.GetBool("FG_AUTO_MIGRATE"),
		// Auth
		OAuth2Login:       viper.GetBool("FG_OAUTH2_LOGIN"),
		SSOEnterprise:     viper.GetBool("FG_SSO_ENTERPRISE"),
		TwoFactorAuth:     viper.GetBool("FG_2FA"),
		EmailVerification: viper.GetBool("FG_EMAIL_VERIFICATION"),
		Turnstile:         viper.GetBool("FG_TURNSTILE"),
		// Feature
		SemanticCache:      viper.GetBool("FG_SEMANTIC_CACHE"),
		ConversationMemory: viper.GetBool("FG_CONVERSATION_MEMORY"),
		PromptSafety:       viper.GetBool("FG_PROMPT_SAFETY"),
		MCPIntegration:     viper.GetBool("FG_MCP"),
		WebhookNotify:      viper.GetBool("FG_WEBHOOK"),
		// Observability
		MetricsUnauthenticated: viper.GetBool("FG_METRICS_UNAUTH"),
		LangfuseTracing:        viper.GetBool("FG_LANGFUSE"),
		SentryErrors:           viper.GetBool("FG_SENTRY"),
		OTelTracing:            viper.GetBool("FG_OTEL"),
	}
	fg.InitMeta()
	return fg
}

// InitMeta builds the runtime maps from struct field values and tags.
// This is called automatically by loadFeatureGates and can also be called
// on a zero-value struct for key resolution (e.g. in LoadFeatureGates).
func (fg *FeatureGates) InitMeta() {
	fg.gates = make(map[string]bool)
	fg.dbValues = make(map[string]bool)
	fg.meta = make(map[string]gateMeta)

	v := reflect.ValueOf(fg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		fg.gates[field.Name] = v.Field(i).Bool()
		fg.meta[field.Name] = gateMeta{
			EnvVar:      field.Tag.Get("env"),
			Category:    field.Tag.Get("gate"),
			Description: field.Tag.Get("desc"),
		}
	}
}

// setFeatureGateDefaults registers Viper defaults for all feature gates.
func setFeatureGateDefaults() {
	// Security (OFF)
	viper.SetDefault("FG_GRAPHQL_INTROSPECTION", false)
	viper.SetDefault("FG_GRAPHQL_PLAYGROUND", false)
	viper.SetDefault("FG_SWAGGER_DOCS", false)
	viper.SetDefault("FG_PPROF_DEBUG", false)
	viper.SetDefault("FG_AUTO_MIGRATE", false)
	// Auth (OFF)
	viper.SetDefault("FG_OAUTH2_LOGIN", false)
	viper.SetDefault("FG_SSO_ENTERPRISE", false)
	viper.SetDefault("FG_2FA", false)
	viper.SetDefault("FG_EMAIL_VERIFICATION", false)
	viper.SetDefault("FG_TURNSTILE", false)
	// Feature (ON)
	viper.SetDefault("FG_SEMANTIC_CACHE", true)
	viper.SetDefault("FG_CONVERSATION_MEMORY", true)
	viper.SetDefault("FG_PROMPT_SAFETY", true)
	viper.SetDefault("FG_MCP", true)
	viper.SetDefault("FG_WEBHOOK", true)
	// Observability (OFF)
	viper.SetDefault("FG_METRICS_UNAUTH", false)
	viper.SetDefault("FG_LANGFUSE", false)
	viper.SetDefault("FG_SENTRY", false)
	viper.SetDefault("FG_OTEL", false)
}

// ─── Runtime Get / Set ──────────────────────────────────────────────

// Get returns the current value of a gate by name (thread-safe).
func (fg *FeatureGates) Get(name string) bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return fg.gates[name]
}

// Set updates a gate value at runtime (thread-safe). Returns an error if
// the gate is locked by an environment variable override.
func (fg *FeatureGates) Set(name string, value bool) error {
	if _, ok := fg.meta[name]; !ok {
		return fmt.Errorf("unknown feature gate: %s", name)
	}
	if fg.IsEnvOverridden(name) {
		return fmt.Errorf("gate %s is locked by environment variable %s", name, fg.meta[name].EnvVar)
	}

	fg.mu.Lock()
	fg.gates[name] = value
	fg.dbValues[name] = value
	fg.mu.Unlock()

	// Sync back to the struct fields for compile-time access
	fg.syncToFields()
	return nil
}

// MergeFromDB applies DB-loaded gate values. Only merges if not overridden by env var.
func (fg *FeatureGates) MergeFromDB(dbGates map[string]bool) {
	fg.mu.Lock()
	for name, val := range dbGates {
		if _, ok := fg.meta[name]; !ok {
			continue // skip unknown gates (stale DB entries)
		}
		fg.dbValues[name] = val
		if !fg.isEnvOverriddenLocked(name) {
			fg.gates[name] = val
		}
	}
	fg.mu.Unlock()
	fg.syncToFields()
}

// syncToFields writes the runtime map back to the struct bool fields.
func (fg *FeatureGates) syncToFields() {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	v := reflect.ValueOf(fg).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		if val, ok := fg.gates[field.Name]; ok {
			v.Field(i).SetBool(val)
		}
	}
}

// ─── Source / Override detection ─────────────────────────────────────

// IsEnvOverridden returns true if the gate's environment variable was
// explicitly set (not just a Viper default).
func (fg *FeatureGates) IsEnvOverridden(name string) bool {
	m, ok := fg.meta[name]
	if !ok {
		return false
	}
	_, exists := os.LookupEnv(m.EnvVar)
	return exists
}

// isEnvOverriddenLocked is the same as IsEnvOverridden but does not lock meta.
// Must only be called while the caller already holds fg.mu.
func (fg *FeatureGates) isEnvOverriddenLocked(name string) bool {
	m, ok := fg.meta[name]
	if !ok {
		return false
	}
	_, exists := os.LookupEnv(m.EnvVar)
	return exists
}

// GetSource returns the authoritative source of a gate's value.
func (fg *FeatureGates) GetSource(name string) string {
	if fg.IsEnvOverridden(name) {
		return "env_override"
	}
	fg.mu.RLock()
	_, inDB := fg.dbValues[name]
	fg.mu.RUnlock()
	if inDB {
		return "database"
	}
	return "default"
}

// DBKey converts a struct field name to a system_configs DB key.
// e.g. "GraphQLIntrospection" -> "fg.graphql_introspection"
func DBKey(fieldName string) string {
	return "fg." + toSnakeCase(fieldName)
}

// FieldNameFromDBKey converts a DB key back to a struct field name.
// e.g. "fg.graphql_introspection" -> "GraphQLIntrospection"
func (fg *FeatureGates) FieldNameFromDBKey(dbKey string) string {
	suffix := strings.TrimPrefix(dbKey, "fg.")
	for name := range fg.meta {
		if toSnakeCase(name) == suffix {
			return name
		}
	}
	return ""
}

// ─── Listing ────────────────────────────────────────────────────────

// ListGates returns all feature gates with their metadata for admin APIs.
func (fg *FeatureGates) ListGates() []GateInfo {
	fg.mu.RLock()
	defer fg.mu.RUnlock()

	var gates []GateInfo
	v := reflect.ValueOf(fg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Type.Kind() != reflect.Bool {
			continue
		}
		name := field.Name
		m := fg.meta[name]
		enabled := fg.gates[name]
		source := "default"
		locked := false
		if fg.isEnvOverriddenLocked(name) {
			source = "env_override"
			locked = true
		} else if _, inDB := fg.dbValues[name]; inDB {
			source = "database"
		}
		gates = append(gates, GateInfo{
			Name:        name,
			Enabled:     enabled,
			Category:    m.Category,
			Description: m.Description,
			EnvVar:      m.EnvVar,
			Source:      source,
			Locked:      locked,
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
			zap.String("source", g.Source),
			zap.String("env", g.EnvVar),
		)
	}
	logger.Info("feature gates summary",
		zap.Int("total", len(gates)),
		zap.Int("enabled", enabled),
		zap.Int("disabled", len(gates)-enabled),
	)
}

// ─── Helpers ────────────────────────────────────────────────────────

// toSnakeCase converts PascalCase/camelCase to snake_case.
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+32)) // toLower
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
