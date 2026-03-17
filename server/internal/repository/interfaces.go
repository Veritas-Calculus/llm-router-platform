// Package repository provides database access layer.
// This file defines interfaces for all repositories, enabling
// mock-based testing of services without real database dependencies.
package repository

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
)

// UserRepo defines the interface for user data access.
type UserRepo interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	GetAll(ctx context.Context) ([]models.User, error)
	Count(ctx context.Context) (int64, error)
	CountActiveUsers(ctx context.Context, since time.Time) (int64, error)
	Search(ctx context.Context, query string) ([]models.User, error)
}

// APIKeyRepo defines the interface for user API key data access.
type APIKeyRepo interface {
	Create(ctx context.Context, key *models.APIKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.APIKey, error)
	GetByKeyHash(ctx context.Context, hash string) (*models.APIKey, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.APIKey, error)
	GetAll(ctx context.Context) ([]models.APIKey, error)
	GetActive(ctx context.Context) ([]models.APIKey, error)
	Update(ctx context.Context, key *models.APIKey) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProviderRepo defines the interface for provider data access.
type ProviderRepo interface {
	Create(ctx context.Context, provider *models.Provider) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Provider, error)
	GetByName(ctx context.Context, name string) (*models.Provider, error)
	GetActive(ctx context.Context) ([]models.Provider, error)
	GetAll(ctx context.Context) ([]models.Provider, error)
	Update(ctx context.Context, provider *models.Provider) error
}

// ProviderAPIKeyRepo defines the interface for provider API key data access.
type ProviderAPIKeyRepo interface {
	Create(ctx context.Context, key *models.ProviderAPIKey) error
	GetActiveByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error)
	GetByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ProviderAPIKey, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ProviderAPIKey, error)
	GetAll(ctx context.Context) ([]models.ProviderAPIKey, error)
	Update(ctx context.Context, key *models.ProviderAPIKey) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ModelRepo defines the interface for model data access.
type ModelRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Model, error)
	GetByName(ctx context.Context, name string) (*models.Model, error)
	GetByProvider(ctx context.Context, providerID uuid.UUID) ([]models.Model, error)
}

// ProxyRepo defines the interface for proxy data access.
type ProxyRepo interface {
	Create(ctx context.Context, proxy *models.Proxy) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Proxy, error)
	GetActive(ctx context.Context) ([]models.Proxy, error)
	GetAll(ctx context.Context) ([]models.Proxy, error)
	Update(ctx context.Context, proxy *models.Proxy) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// UsageLogRepo defines the interface for usage log data access.
type UsageLogRepo interface {
	Create(ctx context.Context, log *models.UsageLog) error
	GetByUserIDAndTimeRange(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]models.UsageLog, error)
	GetByTimeRange(ctx context.Context, start, end time.Time) ([]models.UsageLog, error)
	GetByUserIDAndTimeRangePaginated(ctx context.Context, userID uuid.UUID, start, end time.Time, limit, offset int) ([]models.UsageLog, error)
	GetByTimeRangePaginated(ctx context.Context, start, end time.Time, limit, offset int) ([]models.UsageLog, error)
	AggregateByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) (*UsageSummaryRow, error)
	AggregateDailyByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]DailyUsageRow, error)
	AggregateByProviderByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]ProviderUsageRow, error)
	AggregateByModelByTimeRange(ctx context.Context, userID *uuid.UUID, start, end time.Time) ([]ModelUsageRow, error)
}

// HealthHistoryRepo defines the interface for health history data access.
type HealthHistoryRepo interface {
	Create(ctx context.Context, history *models.HealthHistory) error
	GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID, limit int) ([]models.HealthHistory, error)
	GetRecent(ctx context.Context, targetType string, limit int) ([]models.HealthHistory, error)
}

// ConversationMemoryRepo defines the interface for conversation memory data access.
type ConversationMemoryRepo interface {
	Create(ctx context.Context, memory *models.ConversationMemory) error
	GetByConversation(ctx context.Context, userID uuid.UUID, conversationID string) ([]models.ConversationMemory, error)
	DeleteByConversation(ctx context.Context, userID uuid.UUID, conversationID string) error
	DeleteOldestByConversation(ctx context.Context, userID uuid.UUID, conversationID string, count int) error
	ListConversationIDs(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// AlertRepo defines the interface for alert data access.
type AlertRepo interface {
	Create(ctx context.Context, alert *models.Alert) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Alert, error)
	GetByStatus(ctx context.Context, status string, offset, limit int) ([]models.Alert, int64, error)
	GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.Alert, error)
	Update(ctx context.Context, alert *models.Alert) error
}

// AlertConfigRepo defines the interface for alert config data access.
type AlertConfigRepo interface {
	Create(ctx context.Context, config *models.AlertConfig) error
	GetByTarget(ctx context.Context, targetType string, targetID uuid.UUID) (*models.AlertConfig, error)
	Update(ctx context.Context, config *models.AlertConfig) error
	GetAll(ctx context.Context) ([]models.AlertConfig, error)
}

// BudgetRepo defines the interface for budget data access.
type BudgetRepo interface {
	Upsert(ctx context.Context, budget *models.Budget) error
	GetByUserID(ctx context.Context, userID uuid.UUID) (*models.Budget, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}

// TaskRepo defines the interface for async task data access.
type TaskRepo interface {
	Create(ctx context.Context, task *models.AsyncTask) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AsyncTask, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, status string, limit, offset int) ([]models.AsyncTask, int64, error)
	UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error
	UpdateStatus(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error
	CancelByID(ctx context.Context, id uuid.UUID, completedAt *time.Time) error
	ClaimPending(ctx context.Context, limit int) ([]models.AsyncTask, error)
	RecoverStale(ctx context.Context, staleThreshold time.Time) (int64, error)
}

// AuditLogRepo defines the interface for audit log data access.
type AuditLogRepo interface {
	Create(ctx context.Context, entry *models.AuditLog) error
	Query(ctx context.Context, filter AuditQueryFilter) ([]models.AuditLog, int64, error)
	QueryBatch(ctx context.Context, filter AuditQueryFilter, batchSize, offset int) ([]models.AuditLog, error)
	PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

// Compile-time interface satisfaction checks.
var (
	_ UserRepo               = (*UserRepository)(nil)
	_ APIKeyRepo             = (*APIKeyRepository)(nil)
	_ ProviderRepo           = (*ProviderRepository)(nil)
	_ ProviderAPIKeyRepo     = (*ProviderAPIKeyRepository)(nil)
	_ ModelRepo              = (*ModelRepository)(nil)
	_ ProxyRepo              = (*ProxyRepository)(nil)
	_ UsageLogRepo           = (*UsageLogRepository)(nil)
	_ HealthHistoryRepo      = (*HealthHistoryRepository)(nil)
	_ ConversationMemoryRepo = (*ConversationMemoryRepository)(nil)
	_ AlertRepo              = (*AlertRepository)(nil)
	_ AlertConfigRepo        = (*AlertConfigRepository)(nil)
	_ BudgetRepo             = (*BudgetRepository)(nil)
	_ TaskRepo               = (*TaskRepository)(nil)
	_ AuditLogRepo           = (*AuditLogRepository)(nil)
)
