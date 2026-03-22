package models

// CacheConfig stores advanced semantic cache configuration.
type CacheConfig struct {
	BaseModel
	IsEnabled           bool    `gorm:"default:true" json:"is_enabled"`
	SimilarityThreshold float64 `gorm:"default:0.05" json:"similarity_threshold"`
	DefaultTTLMinutes   int     `gorm:"default:1440" json:"default_ttl_minutes"` // 24 hours
	EmbeddingModel      string  `gorm:"type:varchar(128);default:'text-embedding-3-small'" json:"embedding_model"`
	MaxCacheSize        int     `gorm:"default:10000" json:"max_cache_size"`
}
