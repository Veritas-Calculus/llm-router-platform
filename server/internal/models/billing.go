package models

import "github.com/google/uuid"

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

// Budget represents monthly spending limits for a user.
type Budget struct {
	BaseModel
	UserID          uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"user_id"`
	APIKeyID        *uuid.UUID `gorm:"type:uuid;index" json:"api_key_id,omitempty"`
	MonthlyLimitUSD float64    `gorm:"not null" json:"monthly_limit_usd"`
	AlertThreshold  float64    `gorm:"default:0.8" json:"alert_threshold"`
	EnforceHardLimit bool      `gorm:"default:false" json:"enforce_hard_limit"` // true = block requests on over-budget
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	WebhookURL      string     `json:"webhook_url,omitempty"`
	Email           string     `json:"email,omitempty"`
}
