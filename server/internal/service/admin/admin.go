// Package admin provides miscellaneous admin service-layer operations
// that were previously inlined in GraphQL resolver methods.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/smtp"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"llm-router-platform/internal/config"
	configService "llm-router-platform/internal/service/config"
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
	db           *gorm.DB
	redis        *redis.Client
	config       *config.Config
	logger       *zap.Logger
	systemConfig *configService.Service
}

// NewService creates a new admin service.
func NewService(db *gorm.DB, redis *redis.Client, cfg *config.Config, logger *zap.Logger) *Service {
	return &Service{db: db, redis: redis, config: cfg, logger: logger}
}

// SetSystemConfig injects the config service (avoids circular dependency at init).
func (s *Service) SetSystemConfig(sc *configService.Service) {
	s.systemConfig = sc
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

// ─── SendTestEmail ──────────────────────────────────────────────────────

// SendTestEmail sends a test email using the SMTP settings stored in DB.
func (s *Service) SendTestEmail(ctx context.Context, to string) (bool, error) {
	if s.systemConfig == nil {
		return false, fmt.Errorf("system config service not initialized")
	}

	// Validate email format
	if !strings.Contains(to, "@") || len(to) < 5 || strings.Contains(to, " ") {
		return false, fmt.Errorf("invalid email address")
	}

	// Load email settings from DB
	all, err := s.systemConfig.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to load settings: %w", err)
	}
	emailJSON, ok := all["email"]
	if !ok {
		return false, fmt.Errorf("email settings not configured")
	}

	var emailCfg struct {
		Enabled  bool   `json:"enabled"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		From     string `json:"from"`
		FromName string `json:"fromName"`
	}
	if err := json.Unmarshal([]byte(emailJSON), &emailCfg); err != nil {
		return false, fmt.Errorf("invalid email settings: %w", err)
	}
	if !emailCfg.Enabled || emailCfg.Host == "" {
		return false, fmt.Errorf("email is not enabled or SMTP host not set")
	}

	// Build and send
	addr := fmt.Sprintf("%s:%d", emailCfg.Host, emailCfg.Port)
	from := emailCfg.From
	if from == "" {
		from = emailCfg.Username
	}
	subject := "Test Email from LLM Router Platform"
	body := "This is a test email to verify your SMTP configuration is working correctly."

	safeFromName := strings.ReplaceAll(strings.ReplaceAll(emailCfg.FromName, "\n", ""), "\r", "")
	safeFrom := strings.ReplaceAll(strings.ReplaceAll(from, "\n", ""), "\r", "")
	safeTo := strings.ReplaceAll(strings.ReplaceAll(to, "\n", ""), "\r", "")
	safeSubject := strings.ReplaceAll(strings.ReplaceAll(subject, "\n", ""), "\r", "")

	headers := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n",
		safeFromName, safeFrom, safeTo, safeSubject)
	safeMsg := headers + body

	var auth smtp.Auth
	if emailCfg.Username != "" {
		auth = smtp.PlainAuth("", emailCfg.Username, emailCfg.Password, emailCfg.Host)
	}
	if err := smtp.SendMail(addr, auth, safeFrom, []string{safeTo}, []byte(safeMsg)); err != nil {
		return false, fmt.Errorf("failed to send test email: %w", err)
	}
	return true, nil
}

// ─── TriggerBackup (orchestration) ──────────────────────────────────────

// TriggerBackup validates backup settings, creates a record, and runs pg_dump async.
func (s *Service) TriggerBackup(ctx context.Context) (bool, error) {
	if s.systemConfig == nil {
		return false, fmt.Errorf("system config service not initialized")
	}

	// Load and validate backup settings
	all, err := s.systemConfig.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to load settings: %w", err)
	}
	backupJSON, ok := all["backup"]
	if !ok {
		return false, fmt.Errorf("backup settings not configured")
	}

	var backupCfg struct {
		Enabled  bool   `json:"enabled"`
		S3Bucket string `json:"s3Bucket"`
	}
	if err := json.Unmarshal([]byte(backupJSON), &backupCfg); err != nil {
		return false, fmt.Errorf("invalid backup settings: %w", err)
	}
	if !backupCfg.Enabled || backupCfg.S3Bucket == "" {
		return false, fmt.Errorf("backup is not enabled or S3 bucket not set")
	}

	// Create backup record
	now := time.Now()
	dumpFile := fmt.Sprintf("/tmp/backup-%s.sql.gz", now.Format("20060102-150405"))
	record := models.BackupRecord{
		Type:        "manual",
		Status:      "running",
		StartedAt:   now,
		Destination: fmt.Sprintf("s3://%s/backup-%s.sql.gz", backupCfg.S3Bucket, now.Format("20060102-150405")),
	}

	if err := s.CreateBackupRecord(ctx, &record); err != nil {
		return false, fmt.Errorf("failed to create backup record: %w", err)
	}

	// Run pg_dump asynchronously
	s.RunPgDump(record.ID, dumpFile, now)

	s.logger.Info("manual backup triggered (async)",
		zap.String("bucket", backupCfg.S3Bucket),
		zap.String("recordId", record.ID.String()))

	return true, nil
}

// ─── Organization Members ───────────────────────────────────────────────

// GetUserByEmail looks up a user by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return &u, nil
}

// CheckOrgMemberExists returns true if the user is already a member of the org.
func (s *Service) CheckOrgMemberExists(ctx context.Context, orgID, userID uuid.UUID) bool {
	var existing models.OrganizationMember
	return s.db.WithContext(ctx).Where("org_id = ? AND user_id = ?", orgID, userID).First(&existing).Error == nil
}

// CreateOrgMember creates a new organization member record.
func (s *Service) CreateOrgMember(ctx context.Context, member *models.OrganizationMember) error {
	return s.db.WithContext(ctx).Create(member).Error
}

// GetOrgMemberWithUser fetches a single org member with preloaded User.
func (s *Service) GetOrgMemberWithUser(ctx context.Context, orgID, userID uuid.UUID) (*models.OrganizationMember, error) {
	var member models.OrganizationMember
	if err := s.db.WithContext(ctx).Preload("User").Where("org_id = ? AND user_id = ?", orgID, userID).First(&member).Error; err != nil {
		return nil, fmt.Errorf("member not found")
	}
	return &member, nil
}

// UpdateOrgMemberRole updates a member's role.
func (s *Service) UpdateOrgMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	return s.db.WithContext(ctx).Model(&models.OrganizationMember{}).Where("org_id = ? AND user_id = ?", orgID, userID).Update("role", role).Error
}

// DeleteOrgMember removes a member from the organization.
func (s *Service) DeleteOrgMember(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.db.WithContext(ctx).Where("org_id = ? AND user_id = ?", orgID, userID).Delete(&models.OrganizationMember{}).Error
}

// ListOrgMembers lists all members for an org with preloaded User.
func (s *Service) ListOrgMembers(ctx context.Context, orgID uuid.UUID) ([]models.OrganizationMember, error) {
	var members []models.OrganizationMember
	if err := s.db.WithContext(ctx).Preload("User").Where("org_id = ?", orgID).Find(&members).Error; err != nil {
		return nil, err
	}
	return members, nil
}

// CountAPIKeysForUser returns the count of API keys for a given user.
func (s *Service) CountAPIKeysForUser(ctx context.Context, userID uuid.UUID) int64 {
	var count int64
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("created_by = ?", userID).Count(&count)
	return count
}

// ─── Identity Providers ─────────────────────────────────────────────────

// CreateIdentityProvider creates a new identity provider.
func (s *Service) CreateIdentityProvider(ctx context.Context, idp *models.IdentityProvider) error {
	if err := s.db.WithContext(ctx).Create(idp).Error; err != nil {
		return fmt.Errorf("failed to create identity provider: %w", err)
	}
	s.db.WithContext(ctx).Preload("Organization").First(idp, "id = ?", idp.ID)
	return nil
}

// GetIdentityProvider fetches an identity provider by ID.
func (s *Service) GetIdentityProvider(ctx context.Context, id uuid.UUID) (*models.IdentityProvider, error) {
	var idp models.IdentityProvider
	if err := s.db.WithContext(ctx).First(&idp, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("identity provider not found")
	}
	return &idp, nil
}

// SaveIdentityProvider saves (updates) an identity provider.
func (s *Service) SaveIdentityProvider(ctx context.Context, idp *models.IdentityProvider) error {
	if err := s.db.WithContext(ctx).Save(idp).Error; err != nil {
		return fmt.Errorf("failed to update identity provider: %w", err)
	}
	s.db.WithContext(ctx).Preload("Organization").First(idp, "id = ?", idp.ID)
	return nil
}

// DeleteIdentityProvider deletes an identity provider.
func (s *Service) DeleteIdentityProvider(ctx context.Context, idp *models.IdentityProvider) error {
	return s.db.WithContext(ctx).Delete(idp).Error
}

// ListIdentityProviders lists identity providers for an org with preloaded Org.
func (s *Service) ListIdentityProviders(ctx context.Context, orgID uuid.UUID) ([]models.IdentityProvider, error) {
	var list []models.IdentityProvider
	if err := s.db.WithContext(ctx).Preload("Organization").Where("org_id = ?", orgID).Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// ─── Model Management ───────────────────────────────────────────────────

// CreateModel creates a new model record.
func (s *Service) CreateModel(ctx context.Context, m *models.Model) error {
	return s.db.WithContext(ctx).Create(m).Error
}

// GetModel fetches a model by ID.
func (s *Service) GetModel(ctx context.Context, id uuid.UUID) (*models.Model, error) {
	var m models.Model
	if err := s.db.WithContext(ctx).First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveModel saves (updates) a model.
func (s *Service) SaveModel(ctx context.Context, m *models.Model) error {
	return s.db.WithContext(ctx).Save(m).Error
}

// DeleteModel deletes a model by ID.
func (s *Service) DeleteModel(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&models.Model{}, "id = ?", id).Error
}

// GetProviderWithModels fetches a provider with preloaded models.
func (s *Service) GetProviderWithModels(ctx context.Context, providerID uuid.UUID) (*models.Provider, error) {
	var prov models.Provider
	if err := s.db.WithContext(ctx).Preload("Models").First(&prov, "id = ?", providerID).Error; err != nil {
		return nil, err
	}
	return &prov, nil
}

// FindFirstActiveAPIKey finds the first active API key for a provider.
func (s *Service) FindFirstActiveAPIKey(ctx context.Context, providerID uuid.UUID) (*models.ProviderAPIKey, error) {
	var apiKey models.ProviderAPIKey
	if err := s.db.WithContext(ctx).Where("provider_id = ? AND is_active = ?", providerID, true).Order("priority ASC").First(&apiKey).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// BatchCreateModels creates multiple models.
func (s *Service) BatchCreateModels(ctx context.Context, modelList []models.Model) {
	for i := range modelList {
		s.db.WithContext(ctx).Create(&modelList[i])
	}
}

// ListModelsByProvider returns all models for a given provider.
func (s *Service) ListModelsByProvider(ctx context.Context, providerID uuid.UUID) ([]models.Model, error) {
	var dbModels []models.Model
	if err := s.db.WithContext(ctx).Where("provider_id = ?", providerID).Order("name ASC").Find(&dbModels).Error; err != nil {
		return nil, err
	}
	return dbModels, nil
}

// ─── Infrastructure Counts ──────────────────────────────────────────────

// InfraCounts holds total/active counts for providers, proxies, and API keys.
type InfraCounts struct {
	ProviderTotal  int64
	ProviderActive int64
	ProxyTotal     int64
	ProxyActive    int64
	APIKeyTotal    int64
	APIKeyActive   int64
}

// GetInfraCounts returns aggregated resource counts in a single call.
func (s *Service) GetInfraCounts(ctx context.Context) *InfraCounts {
	c := &InfraCounts{}
	s.db.WithContext(ctx).Model(&models.Provider{}).Count(&c.ProviderTotal)
	s.db.WithContext(ctx).Model(&models.Provider{}).Where("is_active = ?", true).Count(&c.ProviderActive)
	s.db.WithContext(ctx).Model(&models.Proxy{}).Count(&c.ProxyTotal)
	s.db.WithContext(ctx).Model(&models.Proxy{}).Where("is_active = ?", true).Count(&c.ProxyActive)
	s.db.WithContext(ctx).Model(&models.APIKey{}).Count(&c.APIKeyTotal)
	s.db.WithContext(ctx).Model(&models.APIKey{}).Where("is_active = ?", true).Count(&c.APIKeyActive)
	return c
}

// TotalUserCount returns the total number of users.
func (s *Service) TotalUserCount(ctx context.Context) int64 {
	var count int64
	s.db.WithContext(ctx).Model(&models.User{}).Count(&count)
	return count
}

// RevenueStats returns total revenue and period revenue from transactions.
func (s *Service) RevenueStats(ctx context.Context, periodStart time.Time) (totalRevenue, periodRevenue float64) {
	s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("type = ?", "recharge").
		Where("amount > 0").
		Select("COALESCE(SUM(amount), 0)").Scan(&totalRevenue)
	s.db.WithContext(ctx).Model(&models.Transaction{}).
		Where("type = ?", "recharge").
		Where("amount > 0").
		Where("created_at >= ?", periodStart).
		Select("COALESCE(SUM(amount), 0)").Scan(&periodRevenue)
	return
}

// ─── Audit / Error Logs ─────────────────────────────────────────────────

// ListAuditLogs queries audit logs with optional filters and pagination.
func (s *Service) ListAuditLogs(ctx context.Context, page, pageSize int, action, userID, resourceID *string, startDate, endDate *time.Time) ([]models.AuditLog, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.AuditLog{})
	if action != nil && *action != "" {
		query = query.Where("action = ?", *action)
	}
	if userID != nil && *userID != "" {
		query = query.Where("actor_id = ?", *userID)
	}
	if resourceID != nil && *resourceID != "" {
		query = query.Where("resource_id = ?", *resourceID)
	}
	if startDate != nil {
		query = query.Where("created_at >= ?", *startDate)
	}
	if endDate != nil {
		query = query.Where("created_at <= ?", *endDate)
	}

	var total int64
	query.Count(&total)

	var logs []models.AuditLog
	if err := query.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

// ListErrorLogs returns paginated error logs.
func (s *Service) ListErrorLogs(ctx context.Context, page, pageSize int) ([]models.ErrorLog, int64, error) {
	var total int64
	if err := s.db.WithContext(ctx).Model(&models.ErrorLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.ErrorLog
	if err := s.db.WithContext(ctx).Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ─── Invite Codes ───────────────────────────────────────────────────────

// ListInviteCodes returns all invite codes ordered by creation date.
func (s *Service) ListInviteCodes(ctx context.Context) ([]models.InviteCode, error) {
	var codes []models.InviteCode
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&codes).Error; err != nil {
		return nil, err
	}
	return codes, nil
}

// SaveInviteCodeRecord upserts an invite code (used by auto-create SLA config flow).
func (s *Service) SaveInviteCodeRecord(ctx context.Context, code *models.InviteCode) error {
	return s.db.WithContext(ctx).Create(code).Error
}

// ─── Backup Records ─────────────────────────────────────────────────────

// ListBackupRecords returns recent backup records.
func (s *Service) ListBackupRecords(ctx context.Context) ([]models.BackupRecord, error) {
	var records []models.BackupRecord
	if err := s.db.WithContext(ctx).Order("started_at DESC").Limit(20).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// ─── Usage Export ────────────────────────────────────────────────────────

// ExportUserUsageLogs returns usage logs for a specific user since a given time.
func (s *Service) ExportUserUsageLogs(ctx context.Context, userID uuid.UUID, since time.Time) ([]models.UsageLog, error) {
	var logs []models.UsageLog
	if err := s.db.WithContext(ctx).Where("user_id = ? AND created_at >= ?", userID, since).
		Order("created_at DESC").Limit(10000).Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to query usage: %w", err)
	}
	return logs, nil
}

// ─── SLA / Config ───────────────────────────────────────────────────────

// GetSLAConfigByName fetches or auto-creates an SLA config by name.
func (s *Service) GetSLAConfigByName(ctx context.Context, key string) (*models.SystemConfig, error) {
	var conf models.SystemConfig
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&conf).Error; err != nil {
		return nil, err
	}
	return &conf, nil
}

// UpsertSLAConfig creates or updates an SLA config record.
func (s *Service) UpsertSLAConfig(ctx context.Context, conf *models.SystemConfig) error {
	var existing models.SystemConfig
	if err := s.db.WithContext(ctx).Where("key = ?", conf.Key).First(&existing).Error; err != nil {
		return s.db.WithContext(ctx).Create(conf).Error
	}
	conf.ID = existing.ID
	return s.db.WithContext(ctx).Save(conf).Error
}

// ─── Prompt Template Queries ────────────────────────────────────────────

// ListPromptTemplates returns all templates with version counts.
func (s *Service) ListPromptTemplates(ctx context.Context) ([]models.PromptTemplate, error) {
	var templates []models.PromptTemplate
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// GetPromptVersions returns all versions for a template, ordered by version desc.
func (s *Service) GetPromptVersions(ctx context.Context, templateID uuid.UUID) ([]models.PromptVersion, error) {
	var versions []models.PromptVersion
	if err := s.db.WithContext(ctx).Where("template_id = ?", templateID).Order("version DESC").Find(&versions).Error; err != nil {
		return nil, err
	}
	return versions, nil
}

// ─── Routing Rule Counts ────────────────────────────────────────────────

// CountRoutingRules returns the total count of routing rules with optional active filter.
func (s *Service) CountRoutingRules(ctx context.Context, activeOnly bool) int64 {
	var count int64
	q := s.db.WithContext(ctx).Model(&models.RoutingRule{})
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	q.Count(&count)
	return count
}

// ─── System Performance ─────────────────────────────────────────────────

// PerformanceMetrics holds system-wide performance data.
type PerformanceMetrics struct {
	TotalRequests   int64
	FailedRequests  int64
	AvgLatency      float64
	P95Latency      float64
	P99Latency      float64
	ActiveProviders int64
}

// GetPerformanceMetrics calculates system performance metrics from usage logs.
func (s *Service) GetPerformanceMetrics(ctx context.Context, cutoff time.Time) *PerformanceMetrics {
	m := &PerformanceMetrics{}
	s.db.WithContext(ctx).Model(&models.UsageLog{}).Where("created_at >= ?", cutoff).Count(&m.TotalRequests)
	s.db.WithContext(ctx).Model(&models.UsageLog{}).Where("created_at >= ? AND status_code >= ?", cutoff, 400).Count(&m.FailedRequests)
	s.db.WithContext(ctx).Model(&models.UsageLog{}).Where("created_at >= ? AND latency > 0", cutoff).Select("COALESCE(AVG(latency), 0)").Scan(&m.AvgLatency)

	if s.db.Name() == "postgres" {
		_ = s.db.WithContext(ctx).Raw("SELECT COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY latency), 0), COALESCE(percentile_cont(0.99) WITHIN GROUP (ORDER BY latency), 0) FROM usage_logs WHERE created_at >= ? AND latency > 0", cutoff).Row().Scan(&m.P95Latency, &m.P99Latency)
	} else {
		var latencies []float64
		s.db.WithContext(ctx).Model(&models.UsageLog{}).Where("created_at >= ? AND latency > 0", cutoff).Order("latency asc").Pluck("latency", &latencies)
		if len(latencies) > 0 {
			m.P95Latency = latencies[int(float64(len(latencies))*0.95)]
			m.P99Latency = latencies[int(float64(len(latencies))*0.99)]
		}
	}
	s.db.WithContext(ctx).Model(&models.Provider{}).Where("is_active = ?", true).Count(&m.ActiveProviders)
	return m
}

// ─── Misc Queries ───────────────────────────────────────────────────────

// GetProjectOrgIDByAPIKey returns the org_id for an API key's project.
func (s *Service) GetProjectOrgIDByAPIKey(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	return s.GetProjectOrgID(ctx, projectID)
}

// ListProjects returns projects for an org.
func (s *Service) ListProjects(ctx context.Context, orgID uuid.UUID) ([]models.Project, error) {
	var projects []models.Project
	if err := s.db.WithContext(ctx).Where("org_id = ?", orgID).Order("created_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

// GetAPIKeyByIDWithProject fetches an API key by ID.
func (s *Service) GetAPIKeyByIDWithProject(ctx context.Context, keyID uuid.UUID) (*models.APIKey, error) {
	var apiKey models.APIKey
	if err := s.db.WithContext(ctx).Where("id = ?", keyID).First(&apiKey).Error; err != nil {
		return nil, err
	}
	return &apiKey, nil
}

// GetUserUsageSince returns the count of usage logs for a given API key.
func (s *Service) GetUserUsageSince(ctx context.Context, apiKeyID uuid.UUID, since time.Time) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UsageLog{}).
		Where("api_key_id = ? AND created_at >= ?", apiKeyID, since).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
