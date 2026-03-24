package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrganizationRepository handles organization data access.
type OrganizationRepository struct {
	db *gorm.DB
}

// NewOrganizationRepository creates a new organization repository.
func NewOrganizationRepository(db *gorm.DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// DB returns the underlying database connection for advanced queries.
func (r *OrganizationRepository) DB() *gorm.DB {
	return r.db
}

// Create inserts a new organization.
func (r *OrganizationRepository) Create(ctx context.Context, org *models.Organization) error {
	return r.db.WithContext(ctx).Create(org).Error
}

// GetByID retrieves an organization by ID.
func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	var org models.Organization
	if err := r.db.WithContext(ctx).First(&org, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

// Update updates an organization.
func (r *OrganizationRepository) Update(ctx context.Context, org *models.Organization) error {
	return r.db.WithContext(ctx).Save(org).Error
}

// GetByUserID retrieves all organizations that a user owns or is a member of.
func (r *OrganizationRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.Organization, error) {
	var orgs []models.Organization
	if err := r.db.WithContext(ctx).Where("owner_id = ? OR id IN (SELECT org_id FROM organization_members WHERE user_id = ?)", userID, userID).Find(&orgs).Error; err != nil {
		return nil, err
	}
	return orgs, nil
}

// ProjectRepository handles project data access.
type ProjectRepository struct {
	db *gorm.DB
}

// NewProjectRepository creates a new project repository.
func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// Create inserts a new project.
func (r *ProjectRepository) Create(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Create(project).Error
}

// GetByID retrieves a project by ID.
func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	var project models.Project
	if err := r.db.WithContext(ctx).Preload("DlpConfig").First(&project, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

// GetByOrgID retrieves all projects for an organization.
func (r *ProjectRepository) GetByOrgID(ctx context.Context, orgID uuid.UUID) ([]models.Project, error) {
	var projects []models.Project
	if err := r.db.WithContext(ctx).Preload("DlpConfig").Where("org_id = ?", orgID).Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// Update updates a project.
func (r *ProjectRepository) Update(ctx context.Context, project *models.Project) error {
	return r.db.WithContext(ctx).Save(project).Error
}
