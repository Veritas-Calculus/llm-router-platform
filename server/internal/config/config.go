// Package config provides configuration management for the application.
package config

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application.
type Config struct {
	Server        ServerConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Encryption    EncryptionConfig
	Vault         VaultConfig
	ProxyPool     ProxyPoolConfig
	HealthCheck   HealthCheckConfig
	Alert         AlertConfig
	Email         EmailConfig
	JWT           JWTConfig
	RateLimit     RateLimitConfig
	Log           LogConfig
	Admin         AdminConfig
	Security      SecurityConfig
	Registration  RegistrationConfig
	Observability ObservabilityConfig
	Frontend      FrontendConfig
	Stripe        StripeConfig
	OAuth2        OAuth2Config
	Turnstile     TurnstileConfig
	Cleanup       CleanupConfig
}

// SecurityConfig holds API and Gateway security environment settings.
type SecurityConfig struct {
	AdminIPWhitelist string `mapstructure:"admin_ip_whitelist"` // Comma-separated CIDRs/IPs allowed to access admin APIs
}

// OAuth2Config holds OAuth2 social login provider configuration.
type OAuth2Config struct {
	GitHub OAuth2ProviderConfig
	Google OAuth2ProviderConfig
}

// OAuth2ProviderConfig holds a single OAuth2 provider's credentials.
type OAuth2ProviderConfig struct {
	ClientID     string // #nosec G101
	ClientSecret string // #nosec G101
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Port                        string
	Mode                        string
	CORSOrigins                 []string // Allowed CORS origins; empty or ["*"] = allow all
	PprofEnabled                bool     // Opt-in pprof endpoints; default false
	MetricsAllowUnauthenticated bool     // Expose /internal/metrics without auth for Prometheus scraping
	ReadTimeoutSeconds          int      // HTTP server read timeout (default: 30)
	WriteTimeoutSeconds         int      // HTTP server write timeout; must be large for LLM streaming (default: 600)
	AllowLocalProviders         bool     // Allow provider URLs pointing to private/reserved IPs (default: false)
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host                   string
	Port                   string
	User                   string
	Password               string // #nosec G101 -- internal config, never serialized to API responses
	Name                   string
	SSLMode                string
	MaxOpenConns           int    // Maximum number of open connections to the database
	MaxIdleConns           int    // Maximum number of idle connections in the pool
	ConnMaxLifetimeMinutes int    // Maximum lifetime of a connection in minutes
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host       string
	Port       string
	Password   string // #nosec G101 -- internal config, never serialized to API responses
	DB         int
	TLSEnabled bool   // Enable TLS for Redis connection (recommended for production)
}

// EncryptionConfig holds encryption configuration for sensitive data.
type EncryptionConfig struct {
	Key string // #nosec G101 -- 32-byte key for AES-256 encryption, internal config only
}

// VaultConfig holds HashiCorp Vault configuration for centralized secret management.
// When Addr is set, the server uses Vault Transit Engine for encryption instead of local AES.
type VaultConfig struct {
	Addr       string // Vault server address, e.g. "http://vault:8200"
	Token      string // Vault auth token (or use RoleID+SecretID for AppRole)
	TransitKey string // Transit engine key name, e.g. "llm-router"
}

// ProviderConfig holds single provider configuration.
// Used for creating provider clients dynamically.
type ProviderConfig struct {
	APIKey     string // #nosec G101 -- internal config, never serialized to API responses
	BaseURL    string
	HTTPClient HTTPClientProvider // Optional custom HTTP client (e.g., with proxy)
}

// HTTPClientProvider is a function that returns an HTTP client.
// This allows for lazy initialization and custom configurations like proxies.
type HTTPClientProvider func() *http.Client

// ProxyPoolConfig holds proxy pool configuration.
type ProxyPoolConfig struct {
	Enabled bool
	URL     string
}

// HealthCheckConfig holds health check configuration.
type HealthCheckConfig struct {
	Enabled          bool
	Interval         time.Duration
	Timeout          time.Duration
	RetryCount       int
	FailureThreshold int
}

// AlertConfig holds alert notification configuration.
type AlertConfig struct {
	Enabled      bool
	WebhookURL   string
	EmailEnabled bool
}

// EmailConfig holds transactional email configuration.
type EmailConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string // #nosec G101
	From     string
	FromName string
	TLS      bool // Force TLS for SMTP connection (recommended for production)
}

// FrontendConfig holds frontend-related configuration.
type FrontendConfig struct {
	URL string
}

// StripeConfig holds Stripe payment configuration.
type StripeConfig struct {
	Enabled        bool
	SecretKey      string // #nosec G101
	PublishableKey string // #nosec G101
	WebhookSecret  string // #nosec G101
}

// TurnstileConfig holds Cloudflare Turnstile CAPTCHA configuration.
type TurnstileConfig struct {
	Enabled   bool
	SecretKey string // #nosec G101 -- server-side secret for Turnstile verification
	SiteKey   string // Public site key exposed to frontend
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	Secret           string // #nosec G101 -- internal config, never serialized to API responses
	ExpiresIn        time.Duration
	RefreshExpiresIn time.Duration
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerMinute int
}

// LogConfig holds logging configuration.
type LogConfig struct {
	Level  string
	Format string
}

// AdminConfig holds default admin user configuration.
type AdminConfig struct {
	Email    string
	Password string // #nosec G101 -- internal config, never serialized to API responses
	Name     string
}

// RegistrationConfig holds user registration settings.
type RegistrationConfig struct {
	Mode       string // "open", "invite", "closed"
	InviteCode string // Required when Mode == "invite"
}

// CleanupConfig holds data retention settings for periodic cleanup jobs.
type CleanupConfig struct {
	HealthRetentionDays int // Days to retain health check history (default: 30)
	AlertRetentionDays  int // Days to retain resolved alerts (default: 90)
	AuditRetentionDays  int // Days to retain audit log entries (default: 90)
}

// ObservabilityConfig holds observability configuration (e.g. Langfuse, Sentry).
type ObservabilityConfig struct {
	LangfuseEnabled   bool
	LangfusePublicKey string
	LangfuseSecretKey string // #nosec G101
	LangfuseHost      string
	SentryEnabled     bool
	SentryDSN         string
	SentryEnvironment string
	SentrySampleRate  float64
	OTelEnabled       bool
	OTelEndpoint      string // e.g. "localhost:4318" or URL
	OTelServiceName   string // default: "llm-router-platform"
}

// Load reads configuration from environment variables and .env file.
func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	// Try to read .env file, but don't fail if it doesn't exist
	// When running in Docker, environment variables are set directly
	if err := viper.ReadInConfig(); err != nil {
		// Ignore all config file errors - env vars will be used instead
		// This handles both ConfigFileNotFoundError and os.PathError
		_ = err
	}

	setDefaults()

	// Parse CORS origins from comma-separated string
	var corsOrigins []string
	if raw := viper.GetString("CORS_ORIGINS"); raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Port:                        viper.GetString("SERVER_PORT"),
			Mode:                        viper.GetString("GIN_MODE"),
			CORSOrigins:                 corsOrigins,
			PprofEnabled:                viper.GetBool("PPROF_ENABLED"),
			MetricsAllowUnauthenticated: viper.GetBool("METRICS_ALLOW_UNAUTHENTICATED"),
			ReadTimeoutSeconds:          viper.GetInt("SERVER_READ_TIMEOUT_SECONDS"),
			WriteTimeoutSeconds:         viper.GetInt("SERVER_WRITE_TIMEOUT_SECONDS"),
			AllowLocalProviders:         viper.GetBool("ALLOW_LOCAL_PROVIDERS"),
		},
		Database: DatabaseConfig{
			Host:                   viper.GetString("DB_HOST"),
			Port:                   viper.GetString("DB_PORT"),
			User:                   viper.GetString("DB_USER"),
			Password:               viper.GetString("DB_PASSWORD"),
			Name:                   viper.GetString("DB_NAME"),
			SSLMode:                viper.GetString("DB_SSL_MODE"),
			MaxOpenConns:           viper.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:           viper.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetimeMinutes: viper.GetInt("DB_CONN_MAX_LIFETIME_MINUTES"),
		},
		Redis: RedisConfig{
			Host:       viper.GetString("REDIS_HOST"),
			Port:       viper.GetString("REDIS_PORT"),
			Password:   viper.GetString("REDIS_PASSWORD"),
			DB:         viper.GetInt("REDIS_DB"),
			TLSEnabled: viper.GetBool("REDIS_TLS_ENABLED"),
		},
		Encryption: EncryptionConfig{
			Key: viper.GetString("ENCRYPTION_KEY"),
		},
		Vault: VaultConfig{
			Addr:       viper.GetString("VAULT_ADDR"),
			Token:      viper.GetString("VAULT_TOKEN"),
			TransitKey: viper.GetString("VAULT_TRANSIT_KEY"),
		},
		ProxyPool: ProxyPoolConfig{
			Enabled: viper.GetBool("PROXY_POOL_ENABLED"),
			URL:     viper.GetString("PROXY_POOL_URL"),
		},
		HealthCheck: HealthCheckConfig{
			Enabled:          viper.GetBool("HEALTH_CHECK_ENABLED"),
			Interval:         time.Duration(viper.GetInt("HEALTH_CHECK_INTERVAL")) * time.Second,
			Timeout:          time.Duration(viper.GetInt("HEALTH_CHECK_TIMEOUT")) * time.Second,
			RetryCount:       viper.GetInt("HEALTH_CHECK_RETRY_COUNT"),
			FailureThreshold: viper.GetInt("HEALTH_CHECK_FAILURE_THRESHOLD"),
		},
		Alert: AlertConfig{
			Enabled:      viper.GetBool("ALERT_ENABLED"),
			WebhookURL:   viper.GetString("ALERT_WEBHOOK_URL"),
			EmailEnabled: viper.GetBool("ALERT_EMAIL_ENABLED"),
		},
		Email: EmailConfig{
			Enabled:  viper.GetBool("EMAIL_ENABLED"),
			Host:     viper.GetString("EMAIL_SMTP_HOST"),
			Port:     viper.GetInt("EMAIL_SMTP_PORT"),
			Username: viper.GetString("EMAIL_SMTP_USER"),
			Password: viper.GetString("EMAIL_SMTP_PASS"),
			From:     viper.GetString("EMAIL_FROM"),
			FromName: viper.GetString("EMAIL_FROM_NAME"),
			TLS:      viper.GetBool("EMAIL_SMTP_TLS"),
		},
		JWT: JWTConfig{
			Secret:           viper.GetString("JWT_SECRET"),
			ExpiresIn:        viper.GetDuration("JWT_EXPIRES_IN"),
			RefreshExpiresIn: viper.GetDuration("JWT_REFRESH_EXPIRES_IN"),
		},
		RateLimit: RateLimitConfig{
			Enabled:           viper.GetBool("RATE_LIMIT_ENABLED"),
			RequestsPerMinute: viper.GetInt("RATE_LIMIT_REQUESTS_PER_MINUTE"),
		},
		Log: LogConfig{
			Level:  viper.GetString("LOG_LEVEL"),
			Format: viper.GetString("LOG_FORMAT"),
		},
		Admin: AdminConfig{
			Email:    viper.GetString("ADMIN_EMAIL"),
			Password: viper.GetString("ADMIN_PASSWORD"),
			Name:     viper.GetString("ADMIN_NAME"),
		},
		Registration: RegistrationConfig{
			Mode:       viper.GetString("REGISTRATION_MODE"),
			InviteCode: viper.GetString("INVITE_CODE"),
		},
		Observability: ObservabilityConfig{
			LangfuseEnabled:   viper.GetBool("LANGFUSE_ENABLED"),
			LangfusePublicKey: viper.GetString("LANGFUSE_PUBLIC_KEY"),
			LangfuseSecretKey: viper.GetString("LANGFUSE_SECRET_KEY"),
			LangfuseHost:      viper.GetString("LANGFUSE_HOST"),
			SentryEnabled:     viper.GetBool("SENTRY_ENABLED"),
			SentryDSN:         viper.GetString("SENTRY_DSN"),
			SentryEnvironment: viper.GetString("SENTRY_ENVIRONMENT"),
			SentrySampleRate:  viper.GetFloat64("SENTRY_SAMPLE_RATE"),
			OTelEnabled:       viper.GetBool("OTEL_ENABLED"),
			OTelEndpoint:      viper.GetString("OTEL_ENDPOINT"),
			OTelServiceName:   viper.GetString("OTEL_SERVICE_NAME"),
		},
		Frontend: FrontendConfig{
			URL: viper.GetString("FRONTEND_URL"),
		},
		Stripe: StripeConfig{
			Enabled:        viper.GetBool("STRIPE_ENABLED"),
			SecretKey:      viper.GetString("STRIPE_SECRET_KEY"),
			PublishableKey: viper.GetString("STRIPE_PUBLISHABLE_KEY"),
			WebhookSecret:  viper.GetString("STRIPE_WEBHOOK_SECRET"),
		},
		OAuth2: OAuth2Config{
			GitHub: OAuth2ProviderConfig{
				ClientID:     viper.GetString("GITHUB_CLIENT_ID"),
				ClientSecret: viper.GetString("GITHUB_CLIENT_SECRET"),
			},
			Google: OAuth2ProviderConfig{
				ClientID:     viper.GetString("GOOGLE_CLIENT_ID"),
				ClientSecret: viper.GetString("GOOGLE_CLIENT_SECRET"),
			},
		},
		Turnstile: TurnstileConfig{
			Enabled:   viper.GetBool("TURNSTILE_ENABLED"),
			SecretKey: viper.GetString("TURNSTILE_SECRET_KEY"),
			SiteKey:   viper.GetString("TURNSTILE_SITE_KEY"),
		},
		Cleanup: CleanupConfig{
			HealthRetentionDays: viper.GetInt("CLEANUP_HEALTH_RETENTION_DAYS"),
			AlertRetentionDays:  viper.GetInt("CLEANUP_ALERT_RETENTION_DAYS"),
			AuditRetentionDays:  viper.GetInt("CLEANUP_AUDIT_RETENTION_DAYS"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for common misconfigurations.
// Returns an error describing all issues found (not just the first).
func (c *Config) Validate() error {
	var errs []string

	// Server port must be numeric and in valid range
	if c.Server.Port != "" {
		port := 0
		if _, err := fmt.Sscanf(c.Server.Port, "%d", &port); err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Sprintf("SERVER_PORT %q is not a valid port (1-65535)", c.Server.Port))
		}
	}

	// Database port
	if c.Database.Port != "" {
		port := 0
		if _, err := fmt.Sscanf(c.Database.Port, "%d", &port); err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Sprintf("DB_PORT %q is not a valid port (1-65535)", c.Database.Port))
		}
	}

	// Redis port
	if c.Redis.Port != "" {
		port := 0
		if _, err := fmt.Sscanf(c.Redis.Port, "%d", &port); err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Sprintf("REDIS_PORT %q is not a valid port (1-65535)", c.Redis.Port))
		}
	}

	// Email config validation if enabled
	if c.Email.Enabled {
		if c.Email.Host == "" {
			errs = append(errs, "EMAIL_SMTP_HOST is required when email is enabled")
		}
		if c.Email.From == "" {
			errs = append(errs, "EMAIL_FROM is required when email is enabled")
		}
	}

	// JWT expires must be positive
	if c.JWT.ExpiresIn <= 0 {
		errs = append(errs, "JWT_EXPIRES_IN must be a positive duration")
	}
	if c.JWT.RefreshExpiresIn <= 0 {
		errs = append(errs, "JWT_REFRESH_EXPIRES_IN must be a positive duration")
	}

	// Rate limit value
	if c.RateLimit.Enabled && c.RateLimit.RequestsPerMinute <= 0 {
		errs = append(errs, "RATE_LIMIT_REQUESTS_PER_MINUTE must be > 0 when rate limiting is enabled")
	}

	// Log level must be a known value
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	if c.Log.Level != "" && !validLogLevels[strings.ToLower(c.Log.Level)] {
		errs = append(errs, fmt.Sprintf("LOG_LEVEL %q is not valid (debug|info|warn|error|fatal)", c.Log.Level))
	}

	// Registration mode
	validModes := map[string]bool{"open": true, "invite": true, "closed": true}
	if c.Registration.Mode != "" && !validModes[c.Registration.Mode] {
		errs = append(errs, fmt.Sprintf("REGISTRATION_MODE %q is not valid (open|invite|closed)", c.Registration.Mode))
	}

	// Health check interval
	if c.HealthCheck.Enabled && c.HealthCheck.Interval < 5*time.Second {
		errs = append(errs, "HEALTH_CHECK_INTERVAL must be at least 5 seconds")
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(errs, "; "))
}

// setDefaults sets default values for configuration.
func setDefaults() {
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("SERVER_READ_TIMEOUT_SECONDS", 30)
	viper.SetDefault("SERVER_WRITE_TIMEOUT_SECONDS", 600) // Large to support LLM streaming
	viper.SetDefault("GIN_MODE", "release")
	viper.SetDefault("CORS_ORIGINS", "") // Empty = deny by default in production; set to "*" or specific origins
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_SSL_MODE", "require") // Production default; override to "disable" for local dev
	viper.SetDefault("DB_MAX_OPEN_CONNS", 100)
	viper.SetDefault("DB_MAX_IDLE_CONNS", 10)
	viper.SetDefault("DB_CONN_MAX_LIFETIME_MINUTES", 60)
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("REDIS_TLS_ENABLED", false)
	viper.SetDefault("EMAIL_ENABLED", false)
	viper.SetDefault("EMAIL_SMTP_PORT", 587)
	viper.SetDefault("EMAIL_FROM_NAME", "LLM Router")
	viper.SetDefault("FRONTEND_URL", "http://localhost:5173")
	viper.SetDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
	viper.SetDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	viper.SetDefault("OLLAMA_BASE_URL", "http://host.docker.internal:11434")
	viper.SetDefault("LMSTUDIO_BASE_URL", "http://host.docker.internal:1234/v1")
	viper.SetDefault("HEALTH_CHECK_ENABLED", true)
	viper.SetDefault("HEALTH_CHECK_INTERVAL", 60)
	viper.SetDefault("HEALTH_CHECK_TIMEOUT", 10)
	viper.SetDefault("HEALTH_CHECK_RETRY_COUNT", 3)
	viper.SetDefault("HEALTH_CHECK_FAILURE_THRESHOLD", 3)
	viper.SetDefault("JWT_EXPIRES_IN", "1h") // Short-lived access tokens; use refresh tokens for renewal
	viper.SetDefault("JWT_REFRESH_EXPIRES_IN", "168h") // 7 days
	viper.SetDefault("RATE_LIMIT_REQUESTS_PER_MINUTE", 60)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LOG_FORMAT", "json")
	viper.SetDefault("ADMIN_NAME", "Administrator")
	viper.SetDefault("ADMIN_IP_WHITELIST", "")      // Empty = deny by default in strict mode, or open if explicitly handled
	viper.SetDefault("REGISTRATION_MODE", "open") // open by default; set to "invite" or "closed" as needed
	viper.SetDefault("INVITE_CODE", "")           // required when mode=invite
	viper.SetDefault("CLEANUP_HEALTH_RETENTION_DAYS", 30)
	viper.SetDefault("CLEANUP_ALERT_RETENTION_DAYS", 90)
	viper.SetDefault("CLEANUP_AUDIT_RETENTION_DAYS", 90)
	viper.SetDefault("LANGFUSE_ENABLED", false)
	viper.SetDefault("LANGFUSE_HOST", "https://cloud.langfuse.com")
	viper.SetDefault("SENTRY_ENABLED", false)
	viper.SetDefault("SENTRY_ENVIRONMENT", "production")
	viper.SetDefault("SENTRY_SAMPLE_RATE", 1.0)
	viper.SetDefault("STRIPE_ENABLED", false)
	viper.SetDefault("OTEL_ENABLED", false)
	viper.SetDefault("OTEL_ENDPOINT", "")
	viper.SetDefault("OTEL_SERVICE_NAME", "llm-router-platform")
	viper.SetDefault("TURNSTILE_ENABLED", false)
}

// GetDSN returns the database connection string with proper escaping.
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		c.Host, c.User, c.Password, c.Name, c.Port, c.SSLMode)
}

// GetRedisAddr returns the Redis connection address.
func (c *RedisConfig) GetRedisAddr() string {
	return c.Host + ":" + c.Port
}
