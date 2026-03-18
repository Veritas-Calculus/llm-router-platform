package models

import (
	"time"

	"github.com/google/uuid"
)

// Plan represents a subscription tier.
type Plan struct {
	BaseModel
	Name           string  `gorm:"uniqueIndex;not null" json:"name"`
	Description    string  `json:"description"`
	PriceMonth     float64 `gorm:"not null" json:"price_month"` // Monthly price in USD
	TokenLimit     int64   `gorm:"not null" json:"token_limit"` // Tokens per month
	RateLimit      int     `gorm:"not null" json:"rate_limit"`  // Requests per minute
	SupportLevel   string  `gorm:"default:'standard'" json:"support_level"`
	IsActive       bool    `gorm:"default:true" json:"is_active"`
	Features       string  `gorm:"type:text" json:"features"` // JSON string or comma-separated list
}

// Subscription represents a user's active plan.
type Subscription struct {
	BaseModel
	UserID            uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`
	PlanID            uuid.UUID `gorm:"type:uuid;not null" json:"plan_id"`
	Status            string    `gorm:"default:'active'" json:"status"` // active, trialing, canceled, past_due
	CurrentPeriodStart time.Time `json:"current_period_start"`
	CurrentPeriodEnd   time.Time `json:"current_period_end"`
	CancelAtPeriodEnd  bool      `gorm:"default:false" json:"cancel_at_period_end"`
	
	User User `gorm:"foreignKey:UserID" json:"-"`
	Plan Plan `gorm:"foreignKey:PlanID" json:"plan"`
}

// Order represents a payment order.
type Order struct {
	BaseModel
	UserID        uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	PlanID        uuid.UUID `gorm:"type:uuid;index" json:"plan_id"`
	OrderNo       string    `gorm:"uniqueIndex;not null" json:"order_no"`
	Amount        float64   `gorm:"not null" json:"amount"`
	Currency      string    `gorm:"default:'USD'" json:"currency"`
	Status        string    `gorm:"default:'pending'" json:"status"` // pending, paid, failed, expired
	PaymentMethod string    `json:"payment_method"`                  // stripe, alipay, wechat
	ExternalID    string    `gorm:"index" json:"external_id"`        // ID from payment provider (e.g. Stripe Session ID)
}

// Transaction represents any balance movement (recharge, usage deduction, refund).
type Transaction struct {
	BaseModel
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Type        string    `gorm:"not null;index" json:"type"` // recharge, deduction, refund
	Amount      float64   `gorm:"not null" json:"amount"`
	Currency    string    `gorm:"default:'USD'" json:"currency"`
	Balance     float64   `json:"balance"` // Balance AFTER this transaction
	Description string    `json:"description"`
	ReferenceID string    `gorm:"index" json:"reference_id"` // Related Order ID or Usage Log ID
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
	
	// MCP stats
	MCPCallCount   int       `gorm:"default:0" json:"mcp_call_count"`
	MCPErrorCount  int       `gorm:"default:0" json:"mcp_error_count"`

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
