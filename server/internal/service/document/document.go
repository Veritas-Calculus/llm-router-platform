// Package document provides document management services.
package document

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service handles document operations.
type Service struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewService creates a new document service.
func NewService(db *gorm.DB, logger *zap.Logger) *Service {
	return &Service{db: db, logger: logger}
}

// Create creates a new document.
func (s *Service) Create(ctx context.Context, d *models.Document) error {
	return s.db.WithContext(ctx).Create(d).Error
}

// Update updates an existing document.
func (s *Service) Update(ctx context.Context, d *models.Document) error {
	return s.db.WithContext(ctx).Save(d).Error
}

// Delete deletes a document by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.Document{}, "id = ?", id).Error
}

// GetByID retrieves a document by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*models.Document, error) {
	var d models.Document
	if err := s.db.WithContext(ctx).First(&d, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// GetAll retrieves all documents (admin).
func (s *Service) GetAll(ctx context.Context) ([]models.Document, error) {
	var docs []models.Document
	err := s.db.WithContext(ctx).Order("category ASC, sort_order ASC, created_at DESC").Find(&docs).Error
	return docs, err
}

// GetPublished retrieves all published documents (user-facing).
func (s *Service) GetPublished(ctx context.Context) ([]models.Document, error) {
	var docs []models.Document
	err := s.db.WithContext(ctx).
		Where("is_published = ?", true).
		Order("category ASC, sort_order ASC").
		Find(&docs).Error
	return docs, err
}

// GetBySlug retrieves a document by slug.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*models.Document, error) {
	var d models.Document
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}
