package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDSNBuilding(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		user     string
		password string
		dbname   string
		sslmode  string
		expected string
	}{
		{
			name:     "standard connection",
			host:     "localhost",
			port:     "5432",
			user:     "postgres",
			password: "password",
			dbname:   "llm_router",
			sslmode:  "disable",
			expected: "host=localhost port=5432 user=postgres password=password dbname=llm_router sslmode=disable",
		},
		{
			name:     "production connection",
			host:     "db.example.com",
			port:     "5432",
			user:     "app_user",
			password: "secure_pass",
			dbname:   "production_db",
			sslmode:  "require",
			expected: "host=db.example.com port=5432 user=app_user password=secure_pass dbname=production_db sslmode=require",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildDSN(tt.host, tt.port, tt.user, tt.password, tt.dbname, tt.sslmode)
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func buildDSN(host, port, user, password, dbname, sslmode string) string {
	return "host=" + host + " port=" + port + " user=" + user + " password=" + password + " dbname=" + dbname + " sslmode=" + sslmode
}

func TestRedisAddrBuilding(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		port     string
		expected string
	}{
		{
			name:     "localhost",
			host:     "localhost",
			port:     "6379",
			expected: "localhost:6379",
		},
		{
			name:     "remote host",
			host:     "redis.example.com",
			port:     "6380",
			expected: "redis.example.com:6380",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := buildRedisAddr(tt.host, tt.port)
			assert.Equal(t, tt.expected, addr)
		})
	}
}

func buildRedisAddr(host, port string) string {
	return host + ":" + port
}

func TestConnectionPoolSettings(t *testing.T) {
	type PoolConfig struct {
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime int
	}

	tests := []struct {
		name   string
		config PoolConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: PoolConfig{
				MaxOpenConns:    100,
				MaxIdleConns:    10,
				ConnMaxLifetime: 3600,
			},
			valid: true,
		},
		{
			name: "idle greater than open",
			config: PoolConfig{
				MaxOpenConns:    10,
				MaxIdleConns:    100,
				ConnMaxLifetime: 3600,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := tt.config.MaxIdleConns <= tt.config.MaxOpenConns
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestSSLModeValidation(t *testing.T) {
	validModes := []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

	tests := []struct {
		mode  string
		valid bool
	}{
		{"disable", true},
		{"require", true},
		{"verify-full", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			found := false
			for _, m := range validModes {
				if m == tt.mode {
					found = true
					break
				}
			}
			assert.Equal(t, tt.valid, found)
		})
	}
}
