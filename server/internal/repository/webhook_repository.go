package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"llm-router-platform/internal/models"
)

type WebhookRepository interface {
	// Endpoint Operations
	CreateEndpoint(ctx context.Context, endpoint *models.WebhookEndpoint) error
	GetEndpointByID(ctx context.Context, id uuid.UUID) (*models.WebhookEndpoint, error)
	GetEndpointsByProjectID(ctx context.Context, projectID uuid.UUID) ([]*models.WebhookEndpoint, error)
	GetActiveEndpointsByProjectAndEvent(ctx context.Context, projectID uuid.UUID, eventType string) ([]*models.WebhookEndpoint, error)
	UpdateEndpoint(ctx context.Context, endpoint *models.WebhookEndpoint) error
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error

	// Delivery Operations
	CreateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	UpdateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error
	GetDeliveriesByEndpointID(ctx context.Context, endpointID uuid.UUID, limit int) ([]*models.WebhookDelivery, error)
	GetPendingDeliveries(ctx context.Context, limit int) ([]*models.WebhookDelivery, error)
}

type webhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) WebhookRepository {
	return &webhookRepository{db: db}
}

func (r *webhookRepository) CreateEndpoint(ctx context.Context, endpoint *models.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Create(endpoint).Error
}

func (r *webhookRepository) GetEndpointByID(ctx context.Context, id uuid.UUID) (*models.WebhookEndpoint, error) {
	var endpoint models.WebhookEndpoint
	err := r.db.WithContext(ctx).First(&endpoint, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &endpoint, nil
}

func (r *webhookRepository) GetEndpointsByProjectID(ctx context.Context, projectID uuid.UUID) ([]*models.WebhookEndpoint, error) {
	var endpoints []*models.WebhookEndpoint
	err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Find(&endpoints).Error
	return endpoints, err
}

func (r *webhookRepository) GetActiveEndpointsByProjectAndEvent(ctx context.Context, projectID uuid.UUID, eventType string) ([]*models.WebhookEndpoint, error) {
	var endpoints []*models.WebhookEndpoint
	// PostgreSQL JSONB array contains operator ?
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND is_active = ? AND events ? ?", projectID, true, eventType).
		Find(&endpoints).Error
	return endpoints, err
}

func (r *webhookRepository) UpdateEndpoint(ctx context.Context, endpoint *models.WebhookEndpoint) error {
	return r.db.WithContext(ctx).Save(endpoint).Error
}

func (r *webhookRepository) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&models.WebhookEndpoint{}, "id = ?", id).Error
}

func (r *webhookRepository) CreateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	return r.db.WithContext(ctx).Create(delivery).Error
}

func (r *webhookRepository) UpdateDelivery(ctx context.Context, delivery *models.WebhookDelivery) error {
	return r.db.WithContext(ctx).Save(delivery).Error
}

func (r *webhookRepository) GetDeliveriesByEndpointID(ctx context.Context, endpointID uuid.UUID, limit int) ([]*models.WebhookDelivery, error) {
	var deliveries []*models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Where("endpoint_id = ?", endpointID).
		Order("created_at desc").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

func (r *webhookRepository) GetPendingDeliveries(ctx context.Context, limit int) ([]*models.WebhookDelivery, error) {
	var deliveries []*models.WebhookDelivery
	err := r.db.WithContext(ctx).
		Preload("Endpoint"). // Need the endpoint for the URL and Secret
		Where("status = ? AND retry_count < ?", "pending", 3). // Simple criteria
		Order("created_at asc").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}
