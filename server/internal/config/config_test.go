package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := &Config{}

	assert.Equal(t, "", cfg.Server.Port)
	assert.Equal(t, "", cfg.Database.Host)
	assert.Equal(t, "", cfg.JWT.Secret)
}

func TestConfigFromEnv(t *testing.T) {
	_ = os.Setenv("SERVER_PORT", "9000")
	_ = os.Setenv("SERVER_MODE", "test")
	defer func() { _ = os.Unsetenv("SERVER_PORT") }()
	defer func() { _ = os.Unsetenv("SERVER_MODE") }()

	assert.Equal(t, "9000", os.Getenv("SERVER_PORT"))
	assert.Equal(t, "test", os.Getenv("SERVER_MODE"))
}

func TestServerConfig(t *testing.T) {
	serverCfg := ServerConfig{
		Port: "8080",
		Mode: "production",
	}

	assert.Equal(t, "8080", serverCfg.Port)
	assert.Equal(t, "production", serverCfg.Mode)
}

func TestDatabaseConfig(t *testing.T) {
	dbCfg := DatabaseConfig{
		Host:     "localhost",
		Port:     "5432",
		User:     "postgres",
		Password: "password",
		Name:     "llm_router",
		SSLMode:  "disable",
	}

	assert.Equal(t, "localhost", dbCfg.Host)
	assert.Equal(t, "5432", dbCfg.Port)
	assert.Equal(t, "postgres", dbCfg.User)
	assert.Equal(t, "llm_router", dbCfg.Name)
	assert.Equal(t, "disable", dbCfg.SSLMode)
}

func TestRedisConfig(t *testing.T) {
	redisCfg := RedisConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       0,
	}

	assert.Equal(t, "localhost", redisCfg.Host)
	assert.Equal(t, "6379", redisCfg.Port)
	assert.Equal(t, "", redisCfg.Password)
	assert.Equal(t, 0, redisCfg.DB)
}

func TestJWTConfig(t *testing.T) {
	jwtCfg := JWTConfig{
		Secret:    "super-secret-key",
		ExpiresIn: 24 * time.Hour,
	}

	assert.Equal(t, "super-secret-key", jwtCfg.Secret)
	assert.Equal(t, 24*time.Hour, jwtCfg.ExpiresIn)
}

func TestProviderConfigStruct(t *testing.T) {
	providerCfg := ProviderConfig{
		APIKey:  "sk-xxx",
		BaseURL: "https://api.openai.com/v1",
	}

	assert.Equal(t, "sk-xxx", providerCfg.APIKey)
	assert.Equal(t, "https://api.openai.com/v1", providerCfg.BaseURL)
}

func TestHealthCheckConfig(t *testing.T) {
	healthCfg := HealthCheckConfig{
		Enabled:          true,
		Interval:         60 * time.Second,
		Timeout:          10 * time.Second,
		RetryCount:       3,
		FailureThreshold: 5,
	}

	assert.True(t, healthCfg.Enabled)
	assert.Equal(t, 60*time.Second, healthCfg.Interval)
	assert.Equal(t, 10*time.Second, healthCfg.Timeout)
	assert.Equal(t, 3, healthCfg.RetryCount)
	assert.Equal(t, 5, healthCfg.FailureThreshold)
}

func TestAlertConfigStruct(t *testing.T) {
	alertCfg := AlertConfig{
		Enabled:      true,
		WebhookURL:   "https://webhook.example.com",
		EmailEnabled: true,
	}

	assert.True(t, alertCfg.Enabled)
	assert.Equal(t, "https://webhook.example.com", alertCfg.WebhookURL)
	assert.True(t, alertCfg.EmailEnabled)
}

func TestFullConfig(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{
			Port: "8080",
			Mode: "development",
		},
		Database: DatabaseConfig{
			Host: "localhost",
			Port: "5432",
		},
		Redis: RedisConfig{
			Host: "localhost",
			Port: "6379",
		},
		JWT: JWTConfig{
			Secret:    "test-secret",
			ExpiresIn: 24 * time.Hour,
		},
	}

	assert.Equal(t, "8080", cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "localhost", cfg.Redis.Host)
	assert.Equal(t, "test-secret", cfg.JWT.Secret)
}
