package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PromptRepository handles prompt template persistence.
type PromptRepository struct {
	db *gorm.DB
}

// NewPromptRepository creates a new PromptRepository.
func NewPromptRepository(db *gorm.DB) *PromptRepository {
	return &PromptRepository{db: db}
}

// List returns all prompt templates.
func (r *PromptRepository) List(ctx context.Context) ([]models.PromptTemplate, error) {
	var templates []models.PromptTemplate
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&templates).Error
	return templates, err
}

// GetByID returns a prompt template by ID.
func (r *PromptRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.PromptTemplate, error) {
	var t models.PromptTemplate
	err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error
	return &t, err
}

// Create creates a new prompt template.
func (r *PromptRepository) Create(ctx context.Context, t *models.PromptTemplate) error {
	return r.db.WithContext(ctx).Create(t).Error
}

// Update updates an existing prompt template.
func (r *PromptRepository) Update(ctx context.Context, t *models.PromptTemplate) error {
	return r.db.WithContext(ctx).Save(t).Error
}

// Delete soft-deletes a prompt template.
func (r *PromptRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.PromptTemplate{}, "id = ?", id).Error
}

// ListVersions returns all versions for a template, ordered by version desc.
func (r *PromptRepository) ListVersions(ctx context.Context, templateID uuid.UUID) ([]models.PromptVersion, error) {
	var versions []models.PromptVersion
	err := r.db.WithContext(ctx).Where("template_id = ?", templateID).Order("version DESC").Find(&versions).Error
	return versions, err
}

// GetVersionByID returns a specific version.
func (r *PromptRepository) GetVersionByID(ctx context.Context, id uuid.UUID) (*models.PromptVersion, error) {
	var v models.PromptVersion
	err := r.db.WithContext(ctx).First(&v, "id = ?", id).Error
	return &v, err
}

// CreateVersion creates a new version, auto-incrementing the version number.
func (r *PromptRepository) CreateVersion(ctx context.Context, v *models.PromptVersion) error {
	// Get max version number for this template
	var maxVersion int
	r.db.WithContext(ctx).Model(&models.PromptVersion{}).
		Where("template_id = ?", v.TemplateID).
		Select("COALESCE(MAX(version), 0)").
		Scan(&maxVersion)

	v.Version = maxVersion + 1
	return r.db.WithContext(ctx).Create(v).Error
}

// SetActiveVersion updates the template's active version pointer.
func (r *PromptRepository) SetActiveVersion(ctx context.Context, templateID, versionID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.PromptTemplate{}).
		Where("id = ?", templateID).
		Update("active_version_id", versionID).Error
}
