package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	// ModelPatterns is a JSON array of glob patterns used for model→provider routing.
	// Examples: ["gpt-*","o1*","dall-e*","whisper*","tts*"]
	// When empty, falls back to hardcoded heuristics.
	ModelPatterns json.RawMessage `gorm:"type:jsonb" json:"model_patterns,omitempty"`
	Models        []Model         `gorm:"foreignKey:ProviderID" json:"models,omitempty"`
}

// GetModelPatterns deserializes the ModelPatterns JSON field into a string slice.
func (p *Provider) GetModelPatterns() []string {
	if len(p.ModelPatterns) == 0 {
		return nil
	}
	var patterns []string
	if err := json.Unmarshal(p.ModelPatterns, &patterns); err != nil {
		return nil
	}
	return patterns
}

// Model represents an LLM model.
type Model struct {
	BaseModel
	ProviderID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"provider_id"`
	Name                    string          `gorm:"not null" json:"name"`
	DisplayName             string          `json:"display_name"`
	InputPricePer1K         decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"input_price_per_1k"`
	OutputPricePer1K        decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"output_price_per_1k"`
	PricePerSecond          decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"price_per_second,omitempty"` // TTS per-second pricing
	PricePerImage           decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"price_per_image,omitempty"`  // Image generation per-image pricing
	PricePerMinute          decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"price_per_minute,omitempty"` // Video per-minute pricing
	ProviderInputCostPer1K  decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"provider_input_cost_per_1k"`
	ProviderOutputCostPer1K decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"provider_output_cost_per_1k"`
	ProviderCostPerSecond   decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"provider_cost_per_second,omitempty"`
	ProviderCostPerImage    decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"provider_cost_per_image,omitempty"`
	ProviderCostPerMinute   decimal.Decimal `gorm:"type:numeric(20,8);default:0" json:"provider_cost_per_minute,omitempty"`
	MaxTokens               int             `gorm:"default:4096" json:"max_tokens"`
	IsActive                bool            `gorm:"default:true" json:"is_active"`
	Provider                Provider        `gorm:"foreignKey:ProviderID" json:"-"`
}

// ProviderAPIKey represents a provider-specific API key.
type ProviderAPIKey struct {
	BaseModel
	ProviderID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"provider_id"`
	ProxyID         *uuid.UUID `gorm:"type:uuid;index" json:"proxy_id,omitempty"`
	ProxyPoolID     *uuid.UUID `gorm:"type:uuid;index" json:"proxy_pool_id,omitempty"`
	Alias           string     `json:"alias"`
	EncryptedAPIKey string     `gorm:"not null" json:"-"`
	KeyPrefix       string     `json:"key_prefix"`
	IsActive        bool       `gorm:"default:true" json:"is_active"`
	Priority        int        `gorm:"default:1" json:"priority"` // 1 is highest priority
	Weight          float64    `gorm:"default:1.0" json:"weight"`
	RateLimit       int        `gorm:"default:0" json:"rate_limit"`
	UsageCount      int64      `gorm:"default:0" json:"usage_count"`
	LastUsedAt      time.Time  `json:"last_used_at"`
	Provider        Provider   `gorm:"foreignKey:ProviderID" json:"-"`
	Proxy           *Proxy     `gorm:"foreignKey:ProxyID" json:"-"`
	ProxyPool       *ProxyPool `gorm:"foreignKey:ProxyPoolID" json:"-"`
}
