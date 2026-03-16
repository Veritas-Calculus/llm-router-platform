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
	Email                 string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash          string    `gorm:"not null" json:"-"`
	Name                  string    `json:"name"`
	Role                  string    `gorm:"default:user" json:"role"`
	IsActive              bool      `gorm:"default:true" json:"is_active"`
	RequirePasswordChange bool      `gorm:"default:false" json:"require_password_change"`
	APIKeys               []APIKey  `gorm:"foreignKey:UserID" json:"-"`
	LastLoginAt           time.Time `json:"last_login_at"`
	MonthlyTokenLimit     int64     `gorm:"default:0" json:"monthly_token_limit"`   // 0 = unlimited
	MonthlyBudgetUSD      float64   `gorm:"default:0" json:"monthly_budget_usd"`    // 0 = unlimited
	RateLimitPerMinute    int       `gorm:"default:0" json:"rate_limit_per_minute"` // 0 = use global default
	TokensInvalidatedAt   time.Time `json:"-"`                                      // tokens issued before this time are rejected
}

// InviteCode represents a one-time or limited-use invite code for registration.
type InviteCode struct {
	BaseModel
	Code      string     `gorm:"uniqueIndex;not null" json:"code"`
	CreatedBy uuid.UUID  `gorm:"type:uuid;not null" json:"created_by"` // Admin who created this code
	MaxUses   int        `gorm:"default:1" json:"max_uses"`            // 0 = unlimited
	UseCount  int        `gorm:"default:0" json:"use_count"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"` // nil = never expires
	IsActive  bool       `gorm:"default:true" json:"is_active"`
}

// IsValid returns true if the invite code can still be used.
func (ic *InviteCode) IsValid() bool {
	if !ic.IsActive {
		return false
	}
	if ic.MaxUses > 0 && ic.UseCount >= ic.MaxUses {
		return false
	}
	if ic.ExpiresAt != nil && ic.ExpiresAt.Before(time.Now()) {
		return false
	}
	return true
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

// AuditLog records security-relevant events for incident investigation.
type AuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	Action    string    `gorm:"not null;index" json:"action"` // login, login_failed, register, password_change, role_change, user_toggle, apikey_create, apikey_revoke, tokens_invalidated
	ActorID   uuid.UUID `gorm:"type:uuid;index" json:"actor_id"`
	TargetID  uuid.UUID `gorm:"type:uuid;index" json:"target_id"` // affected user (may differ from actor for admin actions)
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Detail    string    `gorm:"type:text" json:"detail"` // JSON metadata
}

// Provider represents an LLM provider.
type Provider struct {
	BaseModel
	Name           string     `gorm:"uniqueIndex;not null" json:"name"`
	BaseURL        string     `gorm:"not null" json:"base_url"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	Priority       int        `gorm:"default:0" json:"priority"`
	Weight         float64    `gorm:"default:1.0" json:"weight"`
	MaxRetries     int        `gorm:"default:3" json:"max_retries"`
	Timeout        int        `gorm:"default:30" json:"timeout"`
	UseProxy       bool       `gorm:"default:false" json:"use_proxy"`
	DefaultProxyID *uuid.UUID `gorm:"type:uuid" json:"default_proxy_id,omitempty"`
	RequiresAPIKey bool       `gorm:"default:true" json:"requires_api_key"`
	Models         []Model    `gorm:"foreignKey:ProviderID" json:"models,omitempty"`
}

// Model represents an LLM model.
type Model struct {
	BaseModel
	ProviderID       uuid.UUID `gorm:"type:uuid;not null;index" json:"provider_id"`
	Name             string    `gorm:"not null" json:"name"`
	DisplayName      string    `json:"display_name"`
	InputPricePer1K  float64   `gorm:"default:0" json:"input_price_per_1k"`
	OutputPricePer1K float64   `gorm:"default:0" json:"output_price_per_1k"`
	PricePerSecond   float64   `gorm:"default:0" json:"price_per_second,omitempty"`  // TTS per-second pricing
	PricePerImage    float64   `gorm:"default:0" json:"price_per_image,omitempty"`   // Image generation per-image pricing
	PricePerMinute   float64   `gorm:"default:0" json:"price_per_minute,omitempty"` // Video per-minute pricing
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
	Priority        int       `gorm:"default:1" json:"priority"` // 1 is highest priority
	Weight          float64   `gorm:"default:1.0" json:"weight"`
	RateLimit       int       `gorm:"default:0" json:"rate_limit"`
	UsageCount      int64     `gorm:"default:0" json:"usage_count"`
	LastUsedAt      time.Time `json:"last_used_at"`
	Provider        Provider  `gorm:"foreignKey:ProviderID" json:"-"`
}

// Proxy represents a proxy server.
type Proxy struct {
	BaseModel
	URL               string     `gorm:"not null" json:"url"`
	Type              string     `gorm:"default:http" json:"type"`
	Username          string     `json:"username,omitempty"`
	EncryptedPassword string     `gorm:"->" json:"-"`              // Read-only, for migration
	Password          string     `gorm:"column:password" json:"-"` // Encrypted password
	Region            string     `json:"region"`
	UpstreamProxyID   *uuid.UUID `gorm:"type:uuid;index" json:"upstream_proxy_id,omitempty"`
	UpstreamProxy     *Proxy     `gorm:"foreignKey:UpstreamProxyID" json:"-"`
	IsActive          bool       `gorm:"default:true" json:"is_active"`
	Weight            float64    `gorm:"default:1.0" json:"weight"`
	SuccessCount      int64      `gorm:"default:0" json:"success_count"`
	FailureCount      int64      `gorm:"default:0" json:"failure_count"`
	AvgLatency        float64    `gorm:"default:0" json:"avg_latency"`
	LastChecked       time.Time  `json:"last_checked"`
}

// HasAuth returns true if the proxy has authentication configured.
func (p *Proxy) HasAuth() bool {
	return p.Username != "" && p.Password != ""
}

// UsageLog represents a single API usage record.
type UsageLog struct {
	BaseModel
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	APIKeyID       uuid.UUID `gorm:"type:uuid;not null;index" json:"api_key_id"`
	ProviderID     uuid.UUID `gorm:"type:uuid;index" json:"provider_id"`
	ModelID        uuid.UUID `gorm:"type:uuid;index" json:"model_id"`
	ModelName      string    `gorm:"index" json:"model_name"`
	ProxyID        uuid.UUID `gorm:"type:uuid;index" json:"proxy_id"`
	RequestTokens  int       `gorm:"column:request_tokens" json:"input_tokens"`
	ResponseTokens int       `gorm:"column:response_tokens" json:"output_tokens"`
	TotalTokens    int       `json:"total_tokens"`
	DurationMs     int64     `json:"duration_ms,omitempty"`      // TTS/Audio duration in milliseconds
	ItemCount      int       `json:"item_count,omitempty"`       // Number of items (images, frames)
	BytesProcessed int64     `json:"bytes_processed,omitempty"` // File size in bytes
	Cost           float64   `json:"cost"`
	Latency        int64     `gorm:"column:latency" json:"latency_ms"`
	StatusCode     int       `json:"status_code"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	IsSuccess      bool      `gorm:"-" json:"is_success"`
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

// Budget represents monthly spending limits for a user.
type Budget struct {
	BaseModel
	UserID          uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	APIKeyID        *uuid.UUID `gorm:"type:uuid;index" json:"api_key_id,omitempty"`
	MonthlyLimitUSD float64    `gorm:"not null" json:"monthly_limit_usd"`
	AlertThreshold  float64    `gorm:"default:0.8" json:"alert_threshold"`
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	WebhookURL      string     `json:"webhook_url,omitempty"`
	Email           string     `json:"email,omitempty"`
}

// AsyncTask represents a long-running asynchronous task.
type AsyncTask struct {
	BaseModel
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Type        string     `gorm:"not null;index" json:"type"`           // "tts", "batch_tts", "video_analysis", "batch_image"
	Status      string     `gorm:"default:pending;index" json:"status"`  // pending, running, completed, failed, cancelled
	Input       string     `gorm:"type:text" json:"input"`               // JSON-encoded input parameters
	Result      string     `gorm:"type:text" json:"result,omitempty"`    // JSON-encoded result
	WebhookURL  string     `json:"webhook_url,omitempty"`                // Callback URL for completion notification
	Progress    int        `gorm:"default:0" json:"progress"`            // 0-100
	Error       string     `json:"error,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
