package webhook

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
	"llm-router-platform/pkg/sanitize"
)

const (
	// maxWebhookRetries matches the repo-side WHERE retry_count < ? cap.
	maxWebhookRetries = 5
	// backoffBase is the first retry's nominal delay; subsequent retries
	// double it (1m, 2m, 4m, 8m, 16m). With jitter the worst-case window
	// before a row is moved to dead-letter is around 30 minutes.
	backoffBase = 1 * time.Minute
	// backoffCap bounds a single retry's wait to avoid pathological cases
	// where retry_count was somehow set high. Also the maximum Retry-After
	// we'll honor — receivers that ask for hours of pause should be
	// dead-lettered through normal retry-count expiry instead.
	backoffCap = 1 * time.Hour
)

// Service defines the interface for webhook operations
type Service interface {
	// Management
	CreateEndpoint(ctx context.Context, projectID uuid.UUID, url string, events []string, isActive bool, description string) (*models.WebhookEndpoint, error)
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

// NewWebhookService constructs the webhook dispatch service.
func NewWebhookService(repo repository.WebhookRepository, logger *zap.Logger) Service {
	return &service{
		repo:   repo,
		logger: logger,
		client: sanitize.SafeHTTPClient(false, 10*time.Second),
	}
}

// generateSecret generates a cryptographically secure random 32-byte hex string
func generateSecret() (string, error) {
	b := make([]byte, 32)
	_, err := cryptorand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// computeBackoff returns the next-attempt delay for a delivery on its
// retryCount-th attempt. Uses 2^n * base with ±25% jitter so a synchronized
// cohort of failures (e.g. all queued in the same DispatchEvent) doesn't all
// retry at the same instant. Capped at backoffCap.
func computeBackoff(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	d := backoffBase << (retryCount - 1)
	if d > backoffCap || d < 0 {
		d = backoffCap
	}
	// Jitter ±25%. crypto/rand here is overkill, but the package already
	// imports it for secret generation and there's no math/rand global to
	// seed at startup.
	var nonce [8]byte
	if _, err := cryptorand.Read(nonce[:]); err == nil {
		r := binary.BigEndian.Uint64(nonce[:])
		// Map to [-0.25, +0.25].
		jit := (float64(r%1000)/1000.0 - 0.5) / 2.0
		d = time.Duration(float64(d) * (1.0 + jit))
	}
	return d
}

// parseRetryAfter reads an RFC 7231 Retry-After header (either a delta-seconds
// integer or an HTTP date) and clamps the result to [0, backoffCap]. Returns
// the zero duration if the header is missing or unparseable.
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		d := time.Duration(secs) * time.Second
		if d > backoffCap {
			d = backoffCap
		}
		return d
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d <= 0 {
			return 0
		}
		if d > backoffCap {
			d = backoffCap
		}
		return d
	}
	return 0
}

func (s *service) CreateEndpoint(ctx context.Context, projectID uuid.UUID, url string, events []string, isActive bool, description string) (*models.WebhookEndpoint, error) {
	if err := sanitize.ValidateWebhookURL(url, false, false); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate webhook secret: %w", err)
	}

	endpoint := &models.WebhookEndpoint{
		ProjectID:   projectID,
		URL:         url,
		Secret:      secret,
		Events:      events,
		IsActive:    isActive,
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
		if err := sanitize.ValidateWebhookURL(url, false, false); err != nil {
			return nil, fmt.Errorf("invalid webhook URL: %w", err)
		}
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
		s.recordFailure(ctx, delivery, 0, err.Error(), "", 0)
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
		s.recordFailure(ctx, delivery, 0, errMsg, "", 0)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	// Read up to 2048 bytes of the response body for debugging. io.ReadFull
	// returns n bytes and one of {nil, io.EOF, io.ErrUnexpectedEOF} — any
	// of those means we have a valid prefix; only an unexpected error from
	// the underlying transport would warrant dropping the bytes.
	bodyBytes := make([]byte, 2048)
	n, readErr := io.ReadFull(resp.Body, bodyBytes)
	var actualBody string
	if n > 0 && (readErr == nil || readErr == io.EOF || readErr == io.ErrUnexpectedEOF) {
		actualBody = string(bodyBytes[:n])
	}

	if resp.StatusCode >= 300 {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		s.recordFailure(ctx, delivery, resp.StatusCode, "", actualBody, retryAfter)
		return
	}
	s.recordSuccess(ctx, delivery, resp.StatusCode, actualBody)
}

func (s *service) recordSuccess(ctx context.Context, delivery *models.WebhookDelivery, statusCode int, body string) {
	delivery.Status = "success"
	delivery.StatusCode = statusCode
	delivery.ErrorMessage = ""
	delivery.ResponseBody = body
	delivery.NextAttemptAt = nil
	if err := s.repo.UpdateDelivery(ctx, delivery); err != nil {
		s.logger.Error("Failed to update webhook delivery status", zap.Error(err), zap.String("id", delivery.ID.String()))
	}
}

// recordFailure persists a failed attempt. If retryCount has reached the cap
// the row moves to "failed" (dead-letter). Otherwise the row stays "pending"
// and next_attempt_at is set to honor either Retry-After (if non-zero) or the
// computed exponential backoff with jitter.
func (s *service) recordFailure(ctx context.Context, delivery *models.WebhookDelivery, statusCode int, errMsg, body string, retryAfter time.Duration) {
	if delivery.RetryCount >= maxWebhookRetries {
		delivery.Status = "failed"
		delivery.NextAttemptAt = nil
	} else {
		delay := retryAfter
		if delay <= 0 {
			delay = computeBackoff(delivery.RetryCount)
		}
		next := time.Now().Add(delay)
		delivery.Status = "pending"
		delivery.NextAttemptAt = &next
	}
	delivery.StatusCode = statusCode
	delivery.ErrorMessage = errMsg
	delivery.ResponseBody = body
	if err := s.repo.UpdateDelivery(ctx, delivery); err != nil {
		s.logger.Error("Failed to update webhook delivery status", zap.Error(err), zap.String("id", delivery.ID.String()))
	}
}
