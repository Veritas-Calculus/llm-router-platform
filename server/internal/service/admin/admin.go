// Package admin provides miscellaneous admin service-layer operations
// that were previously inlined in GraphQL resolver methods.
package admin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service provides miscellaneous admin operations that previously lived
// inline in GraphQL resolvers. It encapsulates *gorm.DB and *redis.Client
// so that resolvers no longer need direct infrastructure access.
type Service struct {
	db     *gorm.DB
	redis  *redis.Client
	config *config.Config
	logger *zap.Logger
}

// NewService creates a new admin service.
func NewService(db *gorm.DB, redis *redis.Client, cfg *config.Config, logger *zap.Logger) *Service {
	return &Service{db: db, redis: redis, config: cfg, logger: logger}
}

// Config returns the application config (read-only convenience for resolvers).
func (s *Service) Config() *config.Config {
	return s.config
}

// DB returns the database connection (backward-compat accessor for resolvers).
func (s *Service) DB() *gorm.DB {
	return s.db
}

// Redis returns the Redis client (backward-compat accessor for resolvers).
func (s *Service) Redis() *redis.Client {
	return s.redis
}

// ─── Alert refetch ──────────────────────────────────────────────────────

// GetAlertByID fetches an alert by ID from the DB.
func (s *Service) GetAlertByID(ctx context.Context, id uuid.UUID) (*models.Alert, error) {
	var alert models.Alert
	if err := s.db.WithContext(ctx).First(&alert, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &alert, nil
}

// ─── Prompt Templates ───────────────────────────────────────────────────

// CreatePromptTemplate creates a new prompt template.
func (s *Service) CreatePromptTemplate(ctx context.Context, t *models.PromptTemplate) error {
	return s.db.WithContext(ctx).Create(t).Error
}

// GetPromptTemplate retrieves a prompt template by ID.
func (s *Service) GetPromptTemplate(ctx context.Context, id uuid.UUID) (*models.PromptTemplate, error) {
	var t models.PromptTemplate
	if err := s.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdatePromptTemplate saves a prompt template.
func (s *Service) UpdatePromptTemplate(ctx context.Context, t *models.PromptTemplate) error {
	return s.db.WithContext(ctx).Save(t).Error
}

// DeletePromptTemplate deletes a prompt template by ID.
func (s *Service) DeletePromptTemplate(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.PromptTemplate{}, "id = ?", id).Error
}

// CountPromptVersions returns the number of versions for a template.
func (s *Service) CountPromptVersions(ctx context.Context, templateID uuid.UUID) (int64, error) {
	var count int64
	s.db.WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", templateID).Count(&count)
	return count, nil
}

// GetMaxPromptVersion returns the highest version number for a template.
func (s *Service) GetMaxPromptVersion(ctx context.Context, templateID uuid.UUID) (int, error) {
	var maxVersion int
	s.db.WithContext(ctx).Model(&models.PromptVersion{}).Where("template_id = ?", templateID).
		Select("COALESCE(MAX(version), 0)").Scan(&maxVersion)
	return maxVersion, nil
}

// CreatePromptVersion creates a new prompt version.
func (s *Service) CreatePromptVersion(ctx context.Context, v *models.PromptVersion) error {
	return s.db.WithContext(ctx).Create(v).Error
}

// GetPromptVersion retrieves a prompt version by ID.
func (s *Service) GetPromptVersion(ctx context.Context, id uuid.UUID) (*models.PromptVersion, error) {
	var v models.PromptVersion
	if err := s.db.WithContext(ctx).First(&v, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

// SetActiveVersion sets the active version for a prompt template.
func (s *Service) SetActiveVersion(ctx context.Context, templateID, versionID uuid.UUID) error {
	return s.db.WithContext(ctx).Model(&models.PromptTemplate{}).Where("id = ?", templateID).
		Update("active_version_id", versionID).Error
}

// ─── Plans ──────────────────────────────────────────────────────────────

// CreatePlan creates a new subscription plan.
func (s *Service) CreatePlan(ctx context.Context, plan *models.Plan) error {
	return s.db.WithContext(ctx).Create(plan).Error
}

// GetPlan retrieves a plan by ID.
func (s *Service) GetPlan(ctx context.Context, id uuid.UUID) (*models.Plan, error) {
	var plan models.Plan
	if err := s.db.WithContext(ctx).First(&plan, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &plan, nil
}

// UpdatePlan saves a plan.
func (s *Service) UpdatePlan(ctx context.Context, plan *models.Plan) error {
	return s.db.WithContext(ctx).Save(plan).Error
}

// ─── Backups ────────────────────────────────────────────────────────────

// CreateBackupRecord creates a backup record and executes pg_dump asynchronously.
func (s *Service) CreateBackupRecord(ctx context.Context, record *models.BackupRecord) error {
	return s.db.WithContext(ctx).Create(record).Error
}

// UpdateBackupRecord updates a backup record (used from async goroutine).
func (s *Service) UpdateBackupRecord(recordID uuid.UUID, updates map[string]interface{}) {
	s.db.Model(&models.BackupRecord{}).Where("id = ?", recordID).Updates(updates)
}

// DatabaseConfig returns the database config for pg_dump operations.
func (s *Service) DatabaseConfig() config.DatabaseConfig {
	return s.config.Database
}

// RunPgDump executes pg_dump in a background goroutine and updates the record.
func (s *Service) RunPgDump(recordID uuid.UUID, dumpFile string, startedAt time.Time) {
	dbCfg := s.config.Database
	go func() {
		pgDumpArgs := []string{
			"-h", dbCfg.Host,
			"-p", dbCfg.Port,
			"-U", dbCfg.User,
			"-d", dbCfg.Name,
			"--no-password",
			"-Fc",
			"-f", dumpFile,
		}

		cmd := exec.Command("pg_dump", pgDumpArgs...) // #nosec G204 -- args from internal config
		cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", dbCfg.Password))

		output, cmdErr := cmd.CombinedOutput()

		completedAt := time.Now()
		updates := map[string]interface{}{
			"completed_at": completedAt,
			"duration_ms":  completedAt.Sub(startedAt).Milliseconds(),
		}

		if cmdErr != nil {
			updates["status"] = "failed"
			updates["error_message"] = fmt.Sprintf("pg_dump failed: %s — %s", cmdErr.Error(), string(output))
			s.logger.Error("backup pg_dump failed",
				zap.Error(cmdErr),
				zap.String("output", string(output)),
				zap.String("recordId", recordID.String()))
		} else {
			if info, statErr := os.Stat(dumpFile); statErr == nil {
				updates["size_bytes"] = info.Size()
			}
			updates["status"] = "success"
			s.logger.Info("backup pg_dump completed",
				zap.String("file", dumpFile),
				zap.String("recordId", recordID.String()))
		}

		s.UpdateBackupRecord(recordID, updates)
	}()
}

// ─── DLP / Invite Codes ─────────────────────────────────────────────────

// CreateInviteCode creates an invite code.
func (s *Service) CreateInviteCode(ctx context.Context, ic *models.InviteCode) error {
	return s.db.WithContext(ctx).Create(ic).Error
}

// ─── System Usage Export ────────────────────────────────────────────────

// ExportSystemUsageLogs returns usage logs with joined user/provider data.
type UsageLogWithUser struct {
	models.UsageLog
	UserEmail    string `gorm:"column:user_email"`
	ProviderName string `gorm:"column:provider_name"`
}

// ExportSystemUsage returns the last 30 days of system-wide usage logs.
func (s *Service) ExportSystemUsage(ctx context.Context) ([]UsageLogWithUser, error) {
	since := time.Now().AddDate(0, 0, -30)
	var logs []UsageLogWithUser
	if err := s.db.WithContext(ctx).Table("usage_logs").
		Select("usage_logs.*, users.email as user_email, providers.name as provider_name").
		Joins("LEFT JOIN users ON users.id = usage_logs.user_id").
		Joins("LEFT JOIN providers ON providers.id = usage_logs.provider_id").
		Where("usage_logs.created_at >= ?", since).
		Order("usage_logs.created_at DESC").Limit(50000).
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to query system usage: %w", err)
	}
	return logs, nil
}

// ─── Rate Limit Status ─────────────────────────────────────────────────

// GetAPIKeyByIDRaw fetches an API key by ID (for rate limit status queries).
func (s *Service) GetAPIKeyByIDRaw(ctx context.Context, keyID string) (*models.APIKey, error) {
	var apiKey models.APIKey
	if err := s.db.WithContext(ctx).Where("id = ?", keyID).First(&apiKey).Error; err != nil {
		return nil, fmt.Errorf("API key not found")
	}
	return &apiKey, nil
}

// RateLimitStatus holds the current rate limit counters for an API key.
type RateLimitStatus struct {
	RpmCurrent    int
	RpmExceeded   bool
	TpmCurrent    int
	TpmExceeded   bool
	DailyCurrent  int
	DailyExceeded bool
}

// GetRateLimitCounters fetches current rate limit counters from Redis for an API key.
func (s *Service) GetRateLimitCounters(ctx context.Context, keyID string, rateLimit int, tokenLimit int64, dailyLimit int) (*RateLimitStatus, error) {
	if s.redis == nil {
		return &RateLimitStatus{}, nil
	}

	rctx := context.Background()
	now := time.Now()

	result := &RateLimitStatus{}

	// 1. RPM
	rpmKey := fmt.Sprintf("rl:key:%s:m", keyID)
	windowStartNano := now.Add(-time.Minute)
	rpmCount, _ := s.redis.ZCount(rctx, rpmKey,
		strconv.FormatInt(windowStartNano.UnixNano(), 10),
		strconv.FormatInt(now.UnixNano(), 10)).Result()

	globalKey := fmt.Sprintf("ratelimit:%s", keyID)
	windowStartMs := now.Add(-time.Minute).UnixMilli()
	globalCount, _ := s.redis.ZCount(rctx, globalKey,
		strconv.FormatInt(windowStartMs, 10),
		strconv.FormatInt(now.UnixMilli(), 10)).Result()

	effectiveRpm := rpmCount
	if globalCount > effectiveRpm {
		effectiveRpm = globalCount
	}
	result.RpmCurrent = int(effectiveRpm)
	if rateLimit > 0 && effectiveRpm >= int64(rateLimit) {
		result.RpmExceeded = true
	}

	// 2. TPM
	tpmKey := fmt.Sprintf("rl:tpm:%s:%d", keyID, now.Unix()/60)
	tpmStr, _ := s.redis.Get(rctx, tpmKey).Result()
	tpmVal, _ := strconv.ParseInt(tpmStr, 10, 32)
	result.TpmCurrent = int(tpmVal)
	if tokenLimit > 0 && tpmVal >= tokenLimit {
		result.TpmExceeded = true
	}

	// 3. Daily
	today := now.Format("2006-01-02")
	dailyKey := fmt.Sprintf("rl:key:%s:d:%s", keyID, today)
	dailyCount, _ := s.redis.Get(rctx, dailyKey).Result()
	dVal, _ := strconv.ParseInt(dailyCount, 10, 32)
	result.DailyCurrent = int(dVal)
	if dailyLimit > 0 && dVal >= int64(dailyLimit) {
		result.DailyExceeded = true
	}

	return result, nil
}

// GetProjectOrgID returns the org_id for a project (for subscription quota checks).
func (s *Service) GetProjectOrgID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var proj struct{ OrgID uuid.UUID }
	if err := s.db.WithContext(ctx).Table("projects").Select("org_id").Where("id = ?", projectID).First(&proj).Error; err != nil {
		return uuid.Nil, err
	}
	return proj.OrgID, nil
}

// ─── Registration Credit Throttle ───────────────────────────────────────

// CheckRegistrationCreditThrottle checks if an IP has exceeded the welcome credit limit.
// Returns true if credit should be granted.
func (s *Service) CheckRegistrationCreditThrottle(ctx context.Context, ip string) bool {
	if s.redis == nil {
		return true
	}
	creditKey := fmt.Sprintf("reg_credit:%s", ip)
	cnt, redisErr := s.redis.Incr(ctx, creditKey).Result()
	if redisErr != nil {
		return true
	}
	if cnt == 1 {
		s.redis.Expire(ctx, creditKey, 24*time.Hour)
	}
	return cnt <= 3
}

// ─── System Config Read/Write ───────────────────────────────────────────

// GetSystemConfig retrieves a system config by key.
func (s *Service) GetSystemConfig(ctx context.Context, key string) (*models.SystemConfig, error) {
	var conf models.SystemConfig
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&conf).Error; err != nil {
		return nil, err
	}
	return &conf, nil
}

// SaveSystemConfig upserts a system config entry.
func (s *Service) SaveSystemConfig(ctx context.Context, conf *models.SystemConfig) error {
	var existing models.SystemConfig
	if err := s.db.WithContext(ctx).Where("key = ?", conf.Key).First(&existing).Error; err != nil {
		return s.db.WithContext(ctx).Create(conf).Error
	}
	conf.ID = existing.ID
	return s.db.WithContext(ctx).Save(conf).Error
}

// ─── Misc DB lookups ────────────────────────────────────────────────────

// GetProjectByID fetches a project (for UpdateProject authorization).
func (s *Service) GetProjectByID(ctx context.Context, projectID uuid.UUID) (*models.Project, error) {
	var project models.Project
	if err := s.db.WithContext(ctx).Where("id = ?", projectID).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

