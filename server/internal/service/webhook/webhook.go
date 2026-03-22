package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
)

// Service defines the interface for webhook operations
type Service interface {
	// Management
	CreateEndpoint(ctx context.Context, projectID uuid.UUID, url string, events []string, description string) (*models.WebhookEndpoint, error)
	GetEndpoints(ctx context.Context, projectID uuid.UUID) ([]*models.WebhookEndpoint, error)
	GetEndpoint(ctx context.Context, id uuid.UUID) (*models.WebhookEndpoint, error)
	UpdateEndpoint(ctx context.Context, id uuid.UUID, url string, events []string, isActive bool, description string) (*models.WebhookEndpoint, error)
	DeleteEndpoint(ctx context.Context, id uuid.UUID) error
	
	// Dispatch
	DispatchEvent(ctx context.Context, projectID uuid.UUID, eventType string, payloadData interface{}) error
	
	// Delivery queries
	GetDeliveries(ctx context.Context, endpointID uuid.UUID, limit int) ([]*models.WebhookDelivery, error)
	
	// Background processing
	ProcessPendingDeliveries(ctx context.Context)
}

type service struct {
	repo   repository.WebhookRepository
	logger *zap.Logger
	client *http.Client
}

func NewWebhookService(repo repository.WebhookRepository, logger *zap.Logger) Service {
	return &service{
		repo:   repo,
		logger: logger,
		client: &http.Client{
			Timeout: 10 * time.Second, // Max time for a webhook dispatch
		},
	}
}

// generateSecret generates a cryptographically secure random 32-byte hex string
func generateSecret() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *service) CreateEndpoint(ctx context.Context, projectID uuid.UUID, url string, events []string, description string) (*models.WebhookEndpoint, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	endpoint := &models.WebhookEndpoint{
		ProjectID:   projectID,
		URL:         url,
		Secret:      secret,
		Events:      events,
		IsActive:    true,
		Description: description,
	}

	if err := s.repo.CreateEndpoint(ctx, endpoint); err != nil {
		return nil, fmt.Errorf("failed to create webhook endpoint: %w", err)
	}

	return endpoint, nil
}

func (s *service) GetEndpoints(ctx context.Context, projectID uuid.UUID) ([]*models.WebhookEndpoint, error) {
	return s.repo.GetEndpointsByProjectID(ctx, projectID)
}

func (s *service) GetEndpoint(ctx context.Context, id uuid.UUID) (*models.WebhookEndpoint, error) {
	return s.repo.GetEndpointByID(ctx, id)
}

func (s *service) UpdateEndpoint(ctx context.Context, id uuid.UUID, url string, events []string, isActive bool, description string) (*models.WebhookEndpoint, error) {
	endpoint, err := s.GetEndpoint(ctx, id)
	if err != nil {
		return nil, err
	}

	if url != "" {
		endpoint.URL = url
	}
	if events != nil {
		endpoint.Events = events
	}
	endpoint.IsActive = isActive
	endpoint.Description = description

	if err := s.repo.UpdateEndpoint(ctx, endpoint); err != nil {
		return nil, fmt.Errorf("failed to update webhook endpoint: %w", err)
	}

	return endpoint, nil
}

func (s *service) DeleteEndpoint(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteEndpoint(ctx, id)
}

func (s *service) DispatchEvent(ctx context.Context, projectID uuid.UUID, eventType string, payloadData interface{}) error {
	// 1. Find active endpoints for this project that are subscribed to eventType
	endpoints, err := s.repo.GetActiveEndpointsByProjectAndEvent(ctx, projectID, eventType)
	if err != nil {
		return fmt.Errorf("failed to find subscribed endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		return nil // No subscribers, nothing to do
	}

	// 2. Serialize payload
	payloadBytes, err := json.Marshal(payloadData)
	if err != nil {
		return fmt.Errorf("failed to marshal payload data: %w", err)
	}
	payloadStr := string(payloadBytes)

	// 3. Create a delivery record for each endpoint
	for _, endpoint := range endpoints {
		delivery := &models.WebhookDelivery{
			EndpointID: endpoint.ID,
			EventType:  eventType,
			Payload:    payloadStr,
			Status:     "pending",
		}
		
		if err := s.repo.CreateDelivery(ctx, delivery); err != nil {
			s.logger.Error("Failed to queue webhook delivery", 
				zap.String("endpointID", endpoint.ID.String()),
				zap.Error(err))
			continue // Don't fail the whole loop just because one failed to save
		}
		
		s.logger.Info("Queued webhook delivery", zap.String("deliveryID", delivery.ID.String()))
	}

	return nil
}

func (s *service) GetDeliveries(ctx context.Context, endpointID uuid.UUID, limit int) ([]*models.WebhookDelivery, error) {
	return s.repo.GetDeliveriesByEndpointID(ctx, endpointID, limit)
}

// computeHMAC calculates HMAC-SHA256 of the payload using the secret
func computeHMAC(payload []byte, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *service) ProcessPendingDeliveries(ctx context.Context) {
	// Fetch a batch of pending deliveries
	deliveries, err := s.repo.GetPendingDeliveries(ctx, 100)
	if err != nil {
		s.logger.Error("Failed to fetch pending webhook deliveries", zap.Error(err))
		return
	}

	if len(deliveries) == 0 {
		return
	}

	s.logger.Info("Processing pending webhook deliveries", zap.Int("count", len(deliveries)))

	for _, delivery := range deliveries {
		// Endpoint might have been deleted, or it wasn't preloaded correctly
		if delivery.Endpoint == nil {
			// Try to load the endpoint
			endpoint, err := s.repo.GetEndpointByID(ctx, delivery.EndpointID)
			if err != nil {
				delivery.Status = "failed"
				delivery.ErrorMessage = "endpoint deleted or not found"
				_ = s.repo.UpdateDelivery(ctx, delivery)
				continue
			}
			delivery.Endpoint = endpoint
		}

		s.executeDelivery(ctx, delivery)
	}
}

func (s *service) executeDelivery(ctx context.Context, delivery *models.WebhookDelivery) {
	delivery.RetryCount++
	
	payloadBytes := []byte(delivery.Payload)
	signature := computeHMAC(payloadBytes, delivery.Endpoint.Secret)

	req, err := http.NewRequestWithContext(ctx, "POST", delivery.Endpoint.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		s.finalizeDelivery(ctx, delivery, "failed", 0, err.Error(), "")
		return
	}

	// Standard webhook headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "LLM-Router-Platform/Webhook")
	req.Header.Set("X-VC-Event", delivery.EventType)
	req.Header.Set("X-VC-Delivery", delivery.ID.String())
	req.Header.Set("X-Hub-Signature-256", "sha256="+signature)

	start := time.Now()
	resp, err := s.client.Do(req)
	duration := time.Since(start)

	if err != nil {
		var errMsg string
		if errors.Is(err, context.DeadlineExceeded) {
			errMsg = fmt.Sprintf("timeout after %v", duration)
		} else {
			errMsg = err.Error()
		}
		
		status := "pending" // Will be retried if < 3
		if delivery.RetryCount >= 3 {
			status = "failed"
		}
		
		s.finalizeDelivery(ctx, delivery, status, 0, errMsg, "")
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read up to 2048 bytes of the response body for debugging
	bodyBytes := make([]byte, 2048)
	n, _ := io.ReadFull(resp.Body, bodyBytes)
	
	var actualBody string
	if n == len(bodyBytes) || err == io.ErrUnexpectedEOF || err == io.EOF {
		actualBody = string(bodyBytes[:n])
	} else if n > 0 {
		actualBody = string(bodyBytes[:n])
	}

	status := "success"
	if resp.StatusCode >= 300 {
		status = "pending"
		if delivery.RetryCount >= 3 {
			status = "failed"
		}
	}

	s.finalizeDelivery(ctx, delivery, status, resp.StatusCode, "", actualBody)
}

func (s *service) finalizeDelivery(ctx context.Context, delivery *models.WebhookDelivery, status string, statusCode int, errorMessage string, responseBody string) {
	delivery.Status = status
	delivery.StatusCode = statusCode
	delivery.ErrorMessage = errorMessage
	delivery.ResponseBody = responseBody

	if err := s.repo.UpdateDelivery(ctx, delivery); err != nil {
		s.logger.Error("Failed to update webhook delivery status", zap.Error(err), zap.String("id", delivery.ID.String()))
	}
}
