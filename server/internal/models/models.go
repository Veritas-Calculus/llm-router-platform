// Package models defines database models for the application.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BaseModel provides common fields for all models.
type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// User represents a platform user.
type User struct {
	BaseModel
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Name         string    `json:"name"`
	Role         string    `gorm:"default:user" json:"role"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	APIKeys      []APIKey  `gorm:"foreignKey:UserID" json:"-"`
	LastLoginAt  time.Time `json:"last_login_at"`
}

// APIKey represents an API key for authentication.
type APIKey struct {
	BaseModel
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	KeyHash    string    `gorm:"not null;uniqueIndex" json:"-"`
	KeyPrefix  string    `gorm:"not null" json:"key_prefix"`
	Name       string    `json:"name"`
	IsActive   bool      `gorm:"default:true" json:"is_active"`
	RateLimit  int       `gorm:"default:1000" json:"rate_limit"`
	DailyLimit int       `gorm:"default:10000" json:"daily_limit"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastUsedAt time.Time `json:"last_used_at"`
	User       User      `gorm:"foreignKey:UserID" json:"-"`
}

// Provider represents an LLM provider.
type Provider struct {
	BaseModel
	Name           string  `gorm:"uniqueIndex;not null" json:"name"`
	BaseURL        string  `gorm:"not null" json:"base_url"`
	IsActive       bool    `gorm:"default:true" json:"is_active"`
	Priority       int     `gorm:"default:0" json:"priority"`
	Weight         float64 `gorm:"default:1.0" json:"weight"`
	MaxRetries     int     `gorm:"default:3" json:"max_retries"`
	Timeout        int     `gorm:"default:30" json:"timeout"`
	UseProxy       bool    `gorm:"default:false" json:"use_proxy"`
	RequiresAPIKey bool    `gorm:"default:true" json:"requires_api_key"`
	Models         []Model `gorm:"foreignKey:ProviderID" json:"models,omitempty"`
}

// Model represents an LLM model.
type Model struct {
	BaseModel
	ProviderID       uuid.UUID `gorm:"type:uuid;not null;index" json:"provider_id"`
	Name             string    `gorm:"not null" json:"name"`
	DisplayName      string    `json:"display_name"`
	InputPricePer1K  float64   `gorm:"default:0" json:"input_price_per_1k"`
	OutputPricePer1K float64   `gorm:"default:0" json:"output_price_per_1k"`
	MaxTokens        int       `gorm:"default:4096" json:"max_tokens"`
	IsActive         bool      `gorm:"default:true" json:"is_active"`
	Provider         Provider  `gorm:"foreignKey:ProviderID" json:"-"`
}

// ProviderAPIKey represents a provider-specific API key.
type ProviderAPIKey struct {
	BaseModel
	ProviderID      uuid.UUID `gorm:"type:uuid;not null;index" json:"provider_id"`
	Alias           string    `json:"alias"`
	EncryptedAPIKey string    `gorm:"not null" json:"-"`
	KeyPrefix       string    `json:"key_prefix"`
	IsActive        bool      `gorm:"default:true" json:"is_active"`
	Weight          float64   `gorm:"default:1.0" json:"weight"`
	RateLimit       int       `gorm:"default:0" json:"rate_limit"`
	UsageCount      int64     `gorm:"default:0" json:"usage_count"`
	LastUsedAt      time.Time `json:"last_used_at"`
	Provider        Provider  `gorm:"foreignKey:ProviderID" json:"-"`
}

// Proxy represents a proxy server.
type Proxy struct {
	BaseModel
	URL          string    `gorm:"not null" json:"url"`
	Type         string    `gorm:"default:http" json:"type"`
	Username     string    `json:"-"`
	Password     string    `json:"-"`
	Region       string    `json:"region"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	Weight       float64   `gorm:"default:1.0" json:"weight"`
	SuccessCount int64     `gorm:"default:0" json:"success_count"`
	FailureCount int64     `gorm:"default:0" json:"failure_count"`
	AvgLatency   float64   `gorm:"default:0" json:"avg_latency"`
	LastChecked  time.Time `json:"last_checked"`
}

// UsageLog represents a single API usage record.
type UsageLog struct {
	BaseModel
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	APIKeyID       uuid.UUID `gorm:"type:uuid;not null;index" json:"api_key_id"`
	ProviderID     uuid.UUID `gorm:"type:uuid;index" json:"provider_id"`
	ModelID        uuid.UUID `gorm:"type:uuid;index" json:"model_id"`
	ProxyID        uuid.UUID `gorm:"type:uuid;index" json:"proxy_id"`
	RequestTokens  int       `json:"request_tokens"`
	ResponseTokens int       `json:"response_tokens"`
	TotalTokens    int       `json:"total_tokens"`
	Cost           float64   `json:"cost"`
	Latency        int64     `json:"latency"`
	StatusCode     int       `json:"status_code"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// HealthHistory records health check results.
type HealthHistory struct {
	BaseModel
	TargetType   string    `gorm:"not null;index" json:"target_type"`
	TargetID     uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	IsHealthy    bool      `json:"is_healthy"`
	ResponseTime int64     `json:"response_time"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CheckedAt    time.Time `gorm:"index" json:"checked_at"`
}

// Alert represents a health alert.
type Alert struct {
	BaseModel
	TargetType     string    `gorm:"not null;index" json:"target_type"`
	TargetID       uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	AlertType      string    `gorm:"not null" json:"alert_type"`
	Message        string    `json:"message"`
	Status         string    `gorm:"default:active;index" json:"status"`
	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
	ResolvedAt     time.Time `json:"resolved_at,omitempty"`
}

// AlertConfig stores alert configuration per target.
type AlertConfig struct {
	BaseModel
	TargetType       string    `gorm:"not null;index" json:"target_type"`
	TargetID         uuid.UUID `gorm:"type:uuid;not null;index" json:"target_id"`
	IsEnabled        bool      `gorm:"default:true" json:"is_enabled"`
	FailureThreshold int       `gorm:"default:3" json:"failure_threshold"`
	WebhookURL       string    `json:"webhook_url,omitempty"`
	Email            string    `json:"email,omitempty"`
}

// ConversationMemory stores conversation context.
type ConversationMemory struct {
	BaseModel
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	ConversationID string    `gorm:"not null;index" json:"conversation_id"`
	Role           string    `gorm:"not null" json:"role"`
	Content        string    `gorm:"type:text" json:"content"`
	TokenCount     int       `json:"token_count"`
	Sequence       int       `gorm:"not null" json:"sequence"`
}
