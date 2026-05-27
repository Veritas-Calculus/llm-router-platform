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

// ModelKind classifies a model row by its capability. A single row has
// exactly one kind — multi-capability models (e.g. chat + embedding) are
// classified by the most specific kind (embedding wins over chat).
type ModelKind string

// ModelKind values. These are the only strings persisted to the
// `models.model_kind` column; see migration 000024 for the CHECK constraint.
const (
	ModelKindChat      ModelKind = "chat"
	ModelKindEmbedding ModelKind = "embedding"
	ModelKindImage     ModelKind = "image"
	ModelKindSTT       ModelKind = "stt"
	ModelKindTTS       ModelKind = "tts"
	ModelKindRerank    ModelKind = "rerank"
	ModelKindUnknown   ModelKind = "unknown"
)

// Model represents an LLM model.
//
// Note on context vs output tokens (audit M-07): `ContextWindow` is the
// total prompt+completion window the model accepts (e.g. 262144 for
// Qwen3.6-35B-A3B). `MaxOutputTokens` is the per-request output cap, which
// is almost always smaller than the context window. The legacy `MaxTokens`
// column is retained for one release as a backcompat shim; new code should
// read `ContextWindow` and `MaxOutputTokens` directly.
type Model struct {
	BaseModel
	ProviderID              uuid.UUID       `gorm:"type:uuid;not null;index" json:"provider_id"`
	Name                    string          `gorm:"not null" json:"name"`
	DisplayName             string          `json:"display_name"`
	ModelKind               ModelKind       `gorm:"type:text;default:'unknown';index" json:"model_kind"`
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
	MaxTokens               int             `gorm:"default:4096" json:"max_tokens"` // Deprecated: kept for backcompat; use ContextWindow.
	ContextWindow           int             `gorm:"default:0" json:"context_window"`
	MaxOutputTokens         *int            `gorm:"default:null" json:"max_output_tokens,omitempty"`
	CatalogWarnings         string          `gorm:"type:text;default:''" json:"catalog_warnings,omitempty"`
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
