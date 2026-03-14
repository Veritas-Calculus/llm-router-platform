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
	ProxyPool     ProxyPoolConfig
	HealthCheck   HealthCheckConfig
	Alert         AlertConfig
	JWT           JWTConfig
	RateLimit     RateLimitConfig
	Log           LogConfig
	Admin         AdminConfig
	Registration  RegistrationConfig
	Observability ObservabilityConfig
}

// ServerConfig holds server-related configuration.
type ServerConfig struct {
	Port         string
	Mode         string
	CORSOrigins  []string // Allowed CORS origins; empty or ["*"] = allow all
	PprofEnabled bool     // Opt-in pprof endpoints; default false
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string // #nosec G101 -- internal config, never serialized to API responses
	Name     string
	SSLMode  string
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string
	Port     string
	Password string // #nosec G101 -- internal config, never serialized to API responses
	DB       int
}

// EncryptionConfig holds encryption configuration for sensitive data.
type EncryptionConfig struct {
	Key string // #nosec G101 -- 32-byte key for AES-256 encryption, internal config only
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
	EmailConfig  EmailConfig
}

// EmailConfig holds email notification configuration.
type EmailConfig struct {
	SMTPHost string
	SMTPPort int
	From     string
	To       string
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	Secret    string // #nosec G101 -- internal config, never serialized to API responses
	ExpiresIn time.Duration
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

// ObservabilityConfig holds observability configuration (e.g. Langfuse).
type ObservabilityConfig struct {
	LangfuseEnabled   bool
	LangfusePublicKey string
	LangfuseSecretKey string // #nosec G101
	LangfuseHost      string
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
			Port:         viper.GetString("SERVER_PORT"),
			Mode:         viper.GetString("GIN_MODE"),
			CORSOrigins:  corsOrigins,
			PprofEnabled: viper.GetBool("PPROF_ENABLED"),
		},
		Database: DatabaseConfig{
			Host:     viper.GetString("DB_HOST"),
			Port:     viper.GetString("DB_PORT"),
			User:     viper.GetString("DB_USER"),
			Password: viper.GetString("DB_PASSWORD"),
			Name:     viper.GetString("DB_NAME"),
			SSLMode:  viper.GetString("DB_SSL_MODE"),
		},
		Redis: RedisConfig{
			Host:     viper.GetString("REDIS_HOST"),
			Port:     viper.GetString("REDIS_PORT"),
			Password: viper.GetString("REDIS_PASSWORD"),
			DB:       viper.GetInt("REDIS_DB"),
		},
		Encryption: EncryptionConfig{
			Key: viper.GetString("ENCRYPTION_KEY"),
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
			EmailConfig: EmailConfig{
				SMTPHost: viper.GetString("ALERT_EMAIL_SMTP_HOST"),
				SMTPPort: viper.GetInt("ALERT_EMAIL_SMTP_PORT"),
				From:     viper.GetString("ALERT_EMAIL_FROM"),
				To:       viper.GetString("ALERT_EMAIL_TO"),
			},
		},
		JWT: JWTConfig{
			Secret:    viper.GetString("JWT_SECRET"),
			ExpiresIn: viper.GetDuration("JWT_EXPIRES_IN"),
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
		},
	}

	return cfg, nil
}

// setDefaults sets default values for configuration.
func setDefaults() {
	viper.SetDefault("SERVER_PORT", "8080")
	viper.SetDefault("GIN_MODE", "release")
	viper.SetDefault("CORS_ORIGINS", "") // Empty = deny by default in production; set to "*" or specific origins
	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", "5432")
	viper.SetDefault("DB_SSL_MODE", "disable")
	viper.SetDefault("REDIS_HOST", "localhost")
	viper.SetDefault("REDIS_PORT", "6379")
	viper.SetDefault("REDIS_DB", 0)
	viper.SetDefault("OPENAI_BASE_URL", "https://api.openai.com/v1")
	viper.SetDefault("ANTHROPIC_BASE_URL", "https://api.anthropic.com")
	viper.SetDefault("OLLAMA_BASE_URL", "http://host.docker.internal:11434")
	viper.SetDefault("LMSTUDIO_BASE_URL", "http://host.docker.internal:1234/v1")
	viper.SetDefault("HEALTH_CHECK_ENABLED", true)
	viper.SetDefault("HEALTH_CHECK_INTERVAL", 60)
	viper.SetDefault("HEALTH_CHECK_TIMEOUT", 10)
	viper.SetDefault("HEALTH_CHECK_RETRY_COUNT", 3)
	viper.SetDefault("HEALTH_CHECK_FAILURE_THRESHOLD", 3)
	viper.SetDefault("JWT_EXPIRES_IN", "24h")
	viper.SetDefault("RATE_LIMIT_REQUESTS_PER_MINUTE", 60)
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("LOG_FORMAT", "json")
	viper.SetDefault("ADMIN_NAME", "Administrator")
	viper.SetDefault("REGISTRATION_MODE", "open") // open, invite, closed
	viper.SetDefault("INVITE_CODE", "")            // required when mode=invite
	viper.SetDefault("LANGFUSE_ENABLED", false)
	viper.SetDefault("LANGFUSE_HOST", "https://cloud.langfuse.com")
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
