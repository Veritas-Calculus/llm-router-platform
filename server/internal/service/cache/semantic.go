package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"llm-router-platform/internal/models"
)

type SemanticCacheService struct {
	db     *gorm.DB
	logger *zap.Logger
	// Threshold for cosine distance. e.g., 0.05
	SimilarityThreshold float64
}

func NewSemanticCacheService(db *gorm.DB, logger *zap.Logger, threshold float64) *SemanticCacheService {
	if threshold <= 0 {
		threshold = 0.05 // default
	}
	return &SemanticCacheService{
		db:                  db,
		logger:              logger,
		SimilarityThreshold: threshold,
	}
}

// HashPrompt creates a SHA-256 hash of the normalized prompt string
func (s *SemanticCacheService) HashPrompt(prompt string) string {
	normalized := strings.TrimSpace(prompt)
	h := sha256.New()
	h.Write([]byte(normalized))
	return hex.EncodeToString(h.Sum(nil))
}

// FindExactMatch looks up a cache entry by exact string hash
func (s *SemanticCacheService) FindExactMatch(ctx context.Context, hash string) (*models.SemanticCache, error) {
	var cache models.SemanticCache
	err := s.db.WithContext(ctx).Where("hash = ?", hash).First(&cache).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Cache miss
		}
		return nil, err
	}

	// Async increment hit count
	go s.db.Model(&cache).Update("hit_count", gorm.Expr("hit_count + 1"))

	return &cache, nil
}

// FindSemanticMatch performs a pgvector cosine distance search
func (s *SemanticCacheService) FindSemanticMatch(ctx context.Context, embedding []float32) (*models.SemanticCache, error) {
	vec := pgvector.NewVector(embedding)

	var cache models.SemanticCache
	// Using cosine distance operator `<=>`
	// Return the closest match if the distance is less than threshold
	err := s.db.WithContext(ctx).
		Where("embedding <=> ? < ?", vec, s.SimilarityThreshold).
		Order(gorm.Expr("embedding <=> ?", vec)).
		First(&cache).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Semantic Cache miss
		}
		return nil, err
	}

	// Async increment hit count
	go s.db.Model(&cache).Update("hit_count", gorm.Expr("hit_count + 1"))

	return &cache, nil
}

// StoreCache saves a new semantic cache entry
func (s *SemanticCacheService) StoreCache(ctx context.Context, hash string, embedding []float32, response interface{}, provider, model string, metadata interface{}) error {
	respBytes, err := json.Marshal(response)
	if err != nil {
		return err
	}

	metaBytes, _ := json.Marshal(metadata)

	cache := models.SemanticCache{
		Hash:      hash,
		Embedding: pgvector.NewVector(embedding),
		Response:  datatypes.JSON(respBytes),
		Provider:  provider,
		Model:     model,
		Metadata:  datatypes.JSON(metaBytes),
		HitCount:  0,
	}

	return s.db.WithContext(ctx).Create(&cache).Error
}

// ListCaches returns a paginated list of caches
func (s *SemanticCacheService) ListCaches(ctx context.Context, limit, offset int) ([]*models.SemanticCache, error) {
	var caches []*models.SemanticCache
	err := s.db.WithContext(ctx).Order("created_at desc").Limit(limit).Offset(offset).Find(&caches).Error
	return caches, err
}

// GetStats returns total caches and total hits
func (s *SemanticCacheService) GetStats(ctx context.Context) (int, int, error) {
	var totalCaches int64
	err := s.db.WithContext(ctx).Model(&models.SemanticCache{}).Count(&totalCaches).Error
	if err != nil {
		return 0, 0, err
	}

	type Result struct {
		TotalHits int
	}
	var res Result
	err = s.db.WithContext(ctx).Model(&models.SemanticCache{}).Select("COALESCE(SUM(hit_count), 0) as total_hits").Scan(&res).Error
	return int(totalCaches), res.TotalHits, err
}

// DeleteCache removes a specific cache entry
func (s *SemanticCacheService) DeleteCache(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Unscoped().Delete(&models.SemanticCache{}, "id = ?", id).Error
}

// DeleteAllCaches clears all semantic caches
func (s *SemanticCacheService) DeleteAllCaches(ctx context.Context) error {
	return s.db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.SemanticCache{}).Error
}

