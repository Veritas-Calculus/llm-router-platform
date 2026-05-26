package webhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type testWebhookRepo struct {
	createdEndpoint *models.WebhookEndpoint
	endpoint        *models.WebhookEndpoint
	updatedEndpoint *models.WebhookEndpoint
	updated         *models.WebhookDelivery
}

func (r *testWebhookRepo) CreateEndpoint(_ context.Context, endpoint *models.WebhookEndpoint) error {
	cp := *endpoint
	r.createdEndpoint = &cp
	return nil
}

func (r *testWebhookRepo) GetEndpointByID(context.Context, uuid.UUID) (*models.WebhookEndpoint, error) {
	if r.endpoint == nil {
		return nil, fmt.Errorf("endpoint not found")
	}
	cp := *r.endpoint
	return &cp, nil
}

func (r *testWebhookRepo) GetEndpointsByProjectID(context.Context, uuid.UUID) ([]*models.WebhookEndpoint, error) {
	return nil, nil
}

func (r *testWebhookRepo) GetActiveEndpointsByProjectAndEvent(context.Context, uuid.UUID, string) ([]*models.WebhookEndpoint, error) {
	return nil, nil
}

func (r *testWebhookRepo) UpdateEndpoint(_ context.Context, endpoint *models.WebhookEndpoint) error {
	cp := *endpoint
	r.updatedEndpoint = &cp
	return nil
}

func (r *testWebhookRepo) DeleteEndpoint(context.Context, uuid.UUID) error {
	return nil
}

func (r *testWebhookRepo) CreateDelivery(context.Context, *models.WebhookDelivery) error {
	return nil
}

func (r *testWebhookRepo) UpdateDelivery(_ context.Context, delivery *models.WebhookDelivery) error {
	cp := *delivery
	r.updated = &cp
	return nil
}

func (r *testWebhookRepo) GetDeliveriesByEndpointID(context.Context, uuid.UUID, int) ([]*models.WebhookDelivery, error) {
	return nil, nil
}

func (r *testWebhookRepo) GetPendingDeliveries(context.Context, int) ([]*models.WebhookDelivery, error) {
	return nil, nil
}

func initWebhookCrypto(t *testing.T) {
	t.Helper()
	if err := crypto.Initialize("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("failed to initialize crypto: %v", err)
	}
}

func TestCreateEndpointStoresEncryptedSecretAndReturnsPlaintext(t *testing.T) {
	initWebhookCrypto(t)

	repo := &testWebhookRepo{}
	svc := &service{repo: repo, logger: zap.NewNop()}

	endpoint, err := svc.CreateEndpoint(context.Background(), uuid.New(), "https://example.com/webhook", []string{"ping"}, true, "Production")
	if err != nil {
		t.Fatalf("CreateEndpoint failed: %v", err)
	}
	if endpoint.Secret == "" {
		t.Fatal("returned plaintext secret is empty")
	}
	if repo.createdEndpoint == nil {
		t.Fatal("endpoint was not persisted")
	}
	if repo.createdEndpoint.Secret == endpoint.Secret {
		t.Fatal("persisted endpoint secret should not be plaintext")
	}
	if !strings.HasPrefix(repo.createdEndpoint.Secret, encryptedSecretPrefix) {
		t.Fatalf("persisted endpoint secret missing encrypted prefix: %q", repo.createdEndpoint.Secret)
	}
	if got := svc.decryptSecret(repo.createdEndpoint.Secret); got != endpoint.Secret {
		t.Fatalf("decrypted persisted secret = %q, want returned plaintext secret", got)
	}
}

func TestUpdateEndpointEncryptsLegacyPlaintextSecret(t *testing.T) {
	initWebhookCrypto(t)

	const legacySecret = "legacy-webhook-secret"
	endpointID := uuid.New()
	repo := &testWebhookRepo{
		endpoint: &models.WebhookEndpoint{
			BaseModel: models.BaseModel{ID: endpointID},
			ProjectID: uuid.New(),
			URL:       "https://example.com/old-webhook",
			Secret:    legacySecret,
			Events:    models.StringArray{"ping"},
			IsActive:  true,
		},
	}
	svc := &service{repo: repo, logger: zap.NewNop()}

	if _, err := svc.UpdateEndpoint(context.Background(), endpointID, "https://example.com/new-webhook", []string{"ping"}, true, "updated"); err != nil {
		t.Fatalf("UpdateEndpoint failed: %v", err)
	}
	if repo.updatedEndpoint == nil {
		t.Fatal("endpoint was not updated")
	}
	if repo.updatedEndpoint.Secret == legacySecret {
		t.Fatal("legacy plaintext secret was not encrypted")
	}
	if !strings.HasPrefix(repo.updatedEndpoint.Secret, encryptedSecretPrefix) {
		t.Fatalf("updated secret missing encrypted prefix: %q", repo.updatedEndpoint.Secret)
	}
	if got := svc.decryptSecret(repo.updatedEndpoint.Secret); got != legacySecret {
		t.Fatalf("decrypted updated secret = %q, want %q", got, legacySecret)
	}
}

func TestUpdateEndpointNormalizesUnprefixedEncryptedSecret(t *testing.T) {
	initWebhookCrypto(t)

	const secret = "old-encrypted-webhook-secret"
	oldEncrypted, err := crypto.Encrypt(secret)
	if err != nil {
		t.Fatalf("failed to encrypt old secret: %v", err)
	}

	endpointID := uuid.New()
	repo := &testWebhookRepo{
		endpoint: &models.WebhookEndpoint{
			BaseModel: models.BaseModel{ID: endpointID},
			ProjectID: uuid.New(),
			URL:       "https://example.com/old-webhook",
			Secret:    oldEncrypted,
			Events:    models.StringArray{"ping"},
			IsActive:  true,
		},
	}
	svc := &service{repo: repo, logger: zap.NewNop()}

	if _, err := svc.UpdateEndpoint(context.Background(), endpointID, "https://example.com/new-webhook", []string{"ping"}, true, "updated"); err != nil {
		t.Fatalf("UpdateEndpoint failed: %v", err)
	}
	if repo.updatedEndpoint == nil {
		t.Fatal("endpoint was not updated")
	}
	if repo.updatedEndpoint.Secret == oldEncrypted {
		t.Fatal("unprefixed encrypted secret was not normalized")
	}
	if !strings.HasPrefix(repo.updatedEndpoint.Secret, encryptedSecretPrefix) {
		t.Fatalf("updated secret missing encrypted prefix: %q", repo.updatedEndpoint.Secret)
	}
	if got := svc.decryptSecret(repo.updatedEndpoint.Secret); got != secret {
		t.Fatalf("decrypted updated secret = %q, want %q", got, secret)
	}
}

func TestExecuteDeliverySendsTimestampedSignatureAndLegacyHeader(t *testing.T) {
	const (
		secret  = "test-webhook-secret"
		payload = `{"hello":"world"}`
		event   = "task.completed"
	)

	deliveryID := uuid.New()
	var gotBody []byte
	var gotSignature, gotTimestamp, gotLegacy, gotEvent, gotDelivery string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
		}
		gotSignature = r.Header.Get("Webhook-Signature")
		gotTimestamp = r.Header.Get("Webhook-Timestamp")
		gotLegacy = r.Header.Get("X-Hub-Signature-256")
		gotEvent = r.Header.Get("X-VC-Event")
		gotDelivery = r.Header.Get("X-VC-Delivery")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	repo := &testWebhookRepo{}
	svc := &service{repo: repo, logger: zap.NewNop(), client: ts.Client()}
	delivery := &models.WebhookDelivery{
		BaseModel: models.BaseModel{ID: deliveryID},
		EventType: event,
		Payload:   payload,
		Status:    "pending",
		Endpoint: &models.WebhookEndpoint{
			BaseModel: models.BaseModel{ID: uuid.New()},
			URL:       ts.URL,
			Secret:    secret,
		},
	}

	svc.executeDelivery(context.Background(), delivery)

	if repo.updated == nil || repo.updated.Status != "success" {
		t.Fatalf("delivery was not marked successful: %#v", repo.updated)
	}
	if string(gotBody) != payload {
		t.Fatalf("payload mismatch: got %q want %q", string(gotBody), payload)
	}
	if gotTimestamp == "" {
		t.Fatal("missing Webhook-Timestamp header")
	}
	if _, err := strconv.ParseInt(gotTimestamp, 10, 64); err != nil {
		t.Fatalf("invalid Webhook-Timestamp %q: %v", gotTimestamp, err)
	}
	expectedTimestamped := computeHMAC([]byte(gotTimestamp+"."+payload), secret)
	if gotSignature != fmt.Sprintf("t=%s,v1=%s", gotTimestamp, expectedTimestamped) {
		t.Fatalf("timestamped signature mismatch: got %q", gotSignature)
	}
	expectedLegacy := "sha256=" + computeHMAC([]byte(payload), secret)
	if gotLegacy != expectedLegacy {
		t.Fatalf("legacy signature mismatch: got %q want %q", gotLegacy, expectedLegacy)
	}
	if gotEvent != event {
		t.Fatalf("event header mismatch: got %q want %q", gotEvent, event)
	}
	if gotDelivery != deliveryID.String() {
		t.Fatalf("delivery header mismatch: got %q want %q", gotDelivery, deliveryID.String())
	}
}

func TestExecuteDeliveryHonorsRetryAfter(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "try later", http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	repo := &testWebhookRepo{}
	svc := &service{repo: repo, logger: zap.NewNop(), client: ts.Client()}
	delivery := &models.WebhookDelivery{
		BaseModel: models.BaseModel{ID: uuid.New()},
		EventType: "task.failed",
		Payload:   `{}`,
		Status:    "pending",
		Endpoint: &models.WebhookEndpoint{
			BaseModel: models.BaseModel{ID: uuid.New()},
			URL:       ts.URL,
			Secret:    "secret",
		},
	}

	start := time.Now()
	svc.executeDelivery(context.Background(), delivery)

	if repo.updated == nil {
		t.Fatal("delivery was not updated")
	}
	if repo.updated.Status != "pending" {
		t.Fatalf("status = %q, want pending", repo.updated.Status)
	}
	if repo.updated.RetryCount != 1 {
		t.Fatalf("retry count = %d, want 1", repo.updated.RetryCount)
	}
	if repo.updated.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", repo.updated.StatusCode, http.StatusServiceUnavailable)
	}
	if repo.updated.NextAttemptAt == nil {
		t.Fatal("next attempt was not scheduled")
	}
	delay := repo.updated.NextAttemptAt.Sub(start)
	if delay < 25*time.Second || delay > 35*time.Second {
		t.Fatalf("retry-after delay = %v, want about 30s", delay)
	}
}

func TestExecuteDeliveryDeadLettersAtRetryCap(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "still failing", http.StatusInternalServerError)
	}))
	defer ts.Close()

	repo := &testWebhookRepo{}
	svc := &service{repo: repo, logger: zap.NewNop(), client: ts.Client()}
	delivery := &models.WebhookDelivery{
		BaseModel:  models.BaseModel{ID: uuid.New()},
		EventType:  "budget.alert",
		Payload:    `{}`,
		Status:     "pending",
		RetryCount: maxWebhookRetries - 1,
		Endpoint: &models.WebhookEndpoint{
			BaseModel: models.BaseModel{ID: uuid.New()},
			URL:       ts.URL,
			Secret:    "secret",
		},
	}

	svc.executeDelivery(context.Background(), delivery)

	if repo.updated == nil {
		t.Fatal("delivery was not updated")
	}
	if repo.updated.Status != "failed" {
		t.Fatalf("status = %q, want failed", repo.updated.Status)
	}
	if repo.updated.RetryCount != maxWebhookRetries {
		t.Fatalf("retry count = %d, want %d", repo.updated.RetryCount, maxWebhookRetries)
	}
	if repo.updated.NextAttemptAt != nil {
		t.Fatalf("next attempt = %v, want nil for dead letter", repo.updated.NextAttemptAt)
	}
}
