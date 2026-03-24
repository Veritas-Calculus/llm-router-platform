package config

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// FeatureGates provides centralized, type-safe feature toggles for security-
// sensitive and operational capabilities.
//
// Gate values are resolved in priority order:
//
//	DB (system_configs, category=featuregate) > Code default
//
// Default philosophy:
//   - Security gates default to OFF (opt-in for safety)
//   - Feature gates default to ON (core capabilities)
//   - Observability gates default to OFF (require external infra)
//
// Gates that overlap with existing AdminSettings (2FA, email verification,
// SSO, OAuth2, Turnstile, Langfuse, Sentry) are intentionally excluded --
// those are managed via their respective Settings tabs.
type FeatureGates struct {
	// ─── Security Gates (default: false) ───────────────────────────────

	// GraphQLIntrospection enables __schema and __type introspection queries.
	GraphQLIntrospection bool `gate:"security" desc:"GraphQL schema introspection (__schema, __type)"`

	// GraphQLPlayground enables the interactive GraphQL playground at GET /graphql.
	GraphQLPlayground bool `gate:"security" desc:"GraphQL interactive playground UI"`

	// SwaggerDocs enables the Swagger API documentation at /swagger/*.
	SwaggerDocs bool `gate:"security" desc:"Swagger/OpenAPI documentation endpoint"`

	// PprofDebug enables pprof profiling endpoints at /debug/pprof/*.
	PprofDebug bool `gate:"security" desc:"Go pprof profiling endpoints"`

	// AutoMigrate enables automatic database schema migration on startup.
	AutoMigrate bool `gate:"security" desc:"GORM AutoMigrate on startup (disable in production)"`

	// ─── Feature Gates (default: true) ─────────────────────────────────

	// SemanticCache enables exact-match and vector-similarity response caching.
	SemanticCache bool `gate:"feature" desc:"Semantic response cache (exact + vector)"`

	// ConversationMemory enables server-side conversation history storage.
	ConversationMemory bool `gate:"feature" desc:"Server-side conversation memory"`

	// PromptSafety enables prompt injection detection (RuleEngine + Llama Guard).
	PromptSafety bool `gate:"feature" desc:"Prompt injection detection"`

	// MCPIntegration enables Model Context Protocol tool-calling support.
	MCPIntegration bool `gate:"feature" desc:"MCP tool integration"`

	// WebhookNotify enables webhook notification delivery.
	WebhookNotify bool `gate:"feature" desc:"Webhook event notifications"`

	// ─── Observability Gates (default: false) ──────────────────────────

	// MetricsUnauthenticated exposes /internal/metrics without JWT auth.
	MetricsUnauthenticated bool `gate:"observability" desc:"Unauthenticated Prometheus metrics endpoint"`

	// OTelTracing enables OpenTelemetry distributed tracing.
	OTelTracing bool `gate:"observability" desc:"OpenTelemetry distributed tracing"`

	// ─── Runtime state (not serialized) ────────────────────────────────
	mu       sync.RWMutex        // protects gates map
	gates    map[string]bool     // canonical runtime state
	dbValues map[string]bool     // values loaded from DB
	meta     map[string]gateMeta // field name -> metadata
}

// gateMeta holds immutable metadata for a single gate, parsed once from struct tags.
type gateMeta struct {
	Category    string
	Description string
}


// GateInfo describes a single feature gate for API/admin consumption.
type GateInfo struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Source      string `json:"source"` // "default" or "database"
}

// loadFeatureGates creates a FeatureGates instance with code defaults.
// DB values are merged later via MergeFromDB after the database is ready.
func loadFeatureGates() *FeatureGates {
	fg := &FeatureGates{
		// Security -- OFF
		GraphQLIntrospection:   false,
		GraphQLPlayground:      false,
		SwaggerDocs:            false,
		PprofDebug:             false,
		AutoMigrate:            false,
		// Feature -- ON
		SemanticCache:          true,
		ConversationMemory:     true,
		PromptSafety:           true,
		MCPIntegration:         true,
		WebhookNotify:          true,
		// Observability -- OFF
		MetricsUnauthenticated: false,
		OTelTracing:            false,
	}
	fg.InitMeta()
	return fg
}

// InitMeta builds the runtime maps from struct field values and tags.
// Called automatically by loadFeatureGates; also callable on a zero-value
// struct for key resolution (e.g. in LoadFeatureGates).
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
			Category:    field.Tag.Get("gate"),
			Description: field.Tag.Get("desc"),
		}
	}
}

// ─── Runtime Get / Set ──────────────────────────────────────────────

// Get returns the current value of a gate by name (thread-safe).
func (fg *FeatureGates) Get(name string) bool {
	fg.mu.RLock()
	defer fg.mu.RUnlock()
	return fg.gates[name]
}

// Set updates a gate value at runtime (thread-safe).
func (fg *FeatureGates) Set(name string, value bool) error {
	if _, ok := fg.meta[name]; !ok {
		return fmt.Errorf("unknown feature gate: %s", name)
	}

	fg.mu.Lock()
	fg.gates[name] = value
	fg.dbValues[name] = value
	fg.mu.Unlock()

	fg.syncToFields()
	return nil
}

// MergeFromDB applies DB-loaded gate values into the runtime state.
func (fg *FeatureGates) MergeFromDB(dbGates map[string]bool) {
	fg.mu.Lock()
	for name, val := range dbGates {
		if _, ok := fg.meta[name]; !ok {
			continue // skip unknown gates (stale DB entries)
		}
		fg.dbValues[name] = val
		fg.gates[name] = val
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

// ─── Source detection ───────────────────────────────────────────────

// GetSource returns the authoritative source of a gate's value.
func (fg *FeatureGates) GetSource(name string) string {
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
		if _, inDB := fg.dbValues[name]; inDB {
			source = "database"
		}
		gates = append(gates, GateInfo{
			Name:        name,
			Enabled:     enabled,
			Category:    m.Category,
			Description: m.Description,
			Source:      source,
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
