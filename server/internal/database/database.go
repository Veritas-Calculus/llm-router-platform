// Package database provides database connection and management.
package database

import (
	"time"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
	"llm-router-platform/pkg/sanitize"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Database wraps the GORM database connection.
type Database struct {
	DB     *gorm.DB
	logger *zap.Logger
}

// New creates a new database connection.
// serverMode controls GORM's SQL log level: "release" = Silent, otherwise = Warn (logs slow queries).
func New(cfg *config.DatabaseConfig, serverMode string, log *zap.Logger) (*Database, error) {
	// In release mode, silence SQL logging.  In dev/test, use Warn level
	// so that GORM logs slow queries (>200ms) which aids diagnosis.
	logLevel := logger.Warn
	if serverMode == "release" {
		logLevel = logger.Silent
	}
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), gormConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMinutes) * time.Minute)

	return &Database{
		DB:     db,
		logger: log,
	}, nil
}

// Migrate runs database migrations.
func (d *Database) Migrate() error {
	// Ensure required PostgreSQL extensions are loaded before AutoMigrate
	// (pgvector is needed for SemanticCache HNSW indexes, pgcrypto for UUID generation)
	for _, ext := range []string{"vector", "pgcrypto"} {
		if err := d.DB.Exec("CREATE EXTENSION IF NOT EXISTS \"" + ext + "\"").Error; err != nil {
			d.logger.Warn("could not create extension (non-fatal)", zap.String("extension", ext), zap.Error(err))
		}
	}

	if err := d.DB.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.OrganizationMember{},
		&models.Project{},
		&models.DlpConfig{},
		&models.APIKey{},
		&models.Provider{},
		&models.Model{},
		&models.ProviderAPIKey{},
		&models.Proxy{},
		&models.UsageLog{},
		&models.HealthHistory{},
		&models.Alert{},
		&models.AlertConfig{},
		&models.ConversationMemory{},
		&models.AuditLog{},
		&models.Budget{},
		&models.AsyncTask{},
		&models.InviteCode{},
		&models.MCPServer{},
		&models.MCPTool{},
		&models.Plan{},
		&models.Subscription{},
		&models.Order{},
		&models.Transaction{},
		&models.SystemConfig{},
		&models.RedeemCode{},
		&models.Announcement{},
		&models.Coupon{},
		&models.Document{},
		&models.PasswordResetToken{},
		&models.EmailVerificationToken{},
		&models.ErrorLog{},
		&models.IntegrationConfig{},
		&models.RoutingRule{},
		&models.SemanticCache{},
		&models.IdentityProvider{},
		&models.WebhookEndpoint{},
		&models.WebhookDelivery{},
		&models.PromptTemplate{},
		&models.PromptVersion{},
		&models.CacheConfig{},
		&models.NotificationChannel{},
		&models.BackupRecord{},
	); err != nil {
		return err
	}

	// Create HNSW index on SemanticCache.embedding via raw SQL
	// (GORM cannot generate valid USING hnsw syntax)
	if err := d.DB.Exec(`CREATE INDEX IF NOT EXISTS idx_semantic_caches_embedding ON semantic_caches USING hnsw (embedding vector_cosine_ops)`).Error; err != nil {
		d.logger.Warn("could not create HNSW index on semantic_caches (pgvector may be unavailable, semantic caching will be disabled)", zap.Error(err))
	}

	return nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// SeedDefaultProviders creates default LLM providers.
func (d *Database) SeedDefaultProviders() error {
	providers := []models.Provider{
		{
			Name:           "openai",
			BaseURL:        "https://api.openai.com/v1",
			IsActive:       true,
			Priority:       10,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        30,
			UseProxy:       false,
			RequiresAPIKey: true,
		},
		{
			Name:           "anthropic",
			BaseURL:        "https://api.anthropic.com",
			IsActive:       true,
			Priority:       9,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        30,
			UseProxy:       false,
			RequiresAPIKey: true,
		},
		{
			Name:           "google",
			BaseURL:        "https://generativelanguage.googleapis.com",
			IsActive:       false,
			Priority:       8,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        30,
			UseProxy:       false,
			RequiresAPIKey: true,
		},
		{
			Name:           "ollama",
			BaseURL:        "http://host.docker.internal:11434/v1",
			IsActive:       true,
			Priority:       5,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        60,
			UseProxy:       false,
			RequiresAPIKey: false,
		},
		{
			Name:           "lmstudio",
			BaseURL:        "http://host.docker.internal:1234/v1",
			IsActive:       true,
			Priority:       5,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        60,
			UseProxy:       false,
			RequiresAPIKey: false,
		},
		{
			Name:           "vllm",
			BaseURL:        "http://host.docker.internal:8000/v1",
			IsActive:       true,
			Priority:       5,
			Weight:         1.0,
			MaxRetries:     3,
			Timeout:        60,
			UseProxy:       false,
			RequiresAPIKey: false,
		},
	}

	for _, provider := range providers {
		var existing models.Provider
		result := d.DB.Where("name = ?", provider.Name).First(&existing)
		if result.Error != nil {
			if err := d.DB.Create(&provider).Error; err != nil {
				d.logger.Error("failed to seed provider", zap.String("name", provider.Name), zap.Error(err))
			}
		}
	}

	return nil
}

// SeedDefaultModels creates default LLM models.
func (d *Database) SeedDefaultModels() error {
	var openaiProvider models.Provider
	if err := d.DB.Where("name = ?", "openai").First(&openaiProvider).Error; err != nil {
		return nil
	}

	modelsList := []models.Model{
		{
			ProviderID:       openaiProvider.ID,
			Name:             "gpt-4",
			DisplayName:      "GPT-4",
			InputPricePer1K:  0.03,
			OutputPricePer1K: 0.06,
			MaxTokens:        8192,
			IsActive:         true,
		},
		{
			ProviderID:       openaiProvider.ID,
			Name:             "gpt-4-turbo",
			DisplayName:      "GPT-4 Turbo",
			InputPricePer1K:  0.01,
			OutputPricePer1K: 0.03,
			MaxTokens:        128000,
			IsActive:         true,
		},
		{
			ProviderID:       openaiProvider.ID,
			Name:             "gpt-3.5-turbo",
			DisplayName:      "GPT-3.5 Turbo",
			InputPricePer1K:  0.0005,
			OutputPricePer1K: 0.0015,
			MaxTokens:        16385,
			IsActive:         true,
		},
	}

	for _, model := range modelsList {
		var existing models.Model
		result := d.DB.Where("name = ? AND provider_id = ?", model.Name, model.ProviderID).First(&existing)
		if result.Error != nil {
			if err := d.DB.Create(&model).Error; err != nil {
				d.logger.Error("failed to seed model", zap.String("name", model.Name), zap.Error(err))
			}
		}
	}

	return nil
}

// SeedDefaultAdminOnly creates the default admin user if it does not already
// exist.  Unlike SeedDefaultAdmin it never overwrites an existing password,
// which is the correct behaviour for production (release) mode — operators
// may have changed the password at runtime and a restart should not reset it.
func (d *Database) SeedDefaultAdminOnly(cfg *config.AdminConfig) error {
	if cfg.Email == "" || cfg.Password == "" {
		d.logger.Info("admin seeding skipped: ADMIN_EMAIL or ADMIN_PASSWORD not set")
		return nil
	}

	var existing models.User
	if d.DB.Where("email = ?", cfg.Email).First(&existing).Error == nil {
		d.logger.Info("admin user already exists, skipping seed (release mode)", zap.String("email", sanitize.MaskEmail(cfg.Email)))
		d.ensureDefaultOrgProject(&existing)
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		d.logger.Error("failed to hash admin password", zap.Error(err))
		return err
	}

	admin := &models.User{
		Email:                 cfg.Email,
		PasswordHash:          string(hashedPassword),
		Name:                  cfg.Name,
		Role:                  "admin",
		IsActive:              true,
		EmailVerified:         true,
		RequirePasswordChange: false,
	}

	if err := d.DB.Create(admin).Error; err != nil {
		d.logger.Error("failed to create admin user", zap.Error(err))
		return err
	}

	d.ensureDefaultOrgProject(admin)
	d.logger.Info("default admin user created (release mode)", zap.String("email", sanitize.MaskEmail(cfg.Email)))
	return nil
}

// SeedDefaultAdmin creates a default admin user if configured.
func (d *Database) SeedDefaultAdmin(cfg *config.AdminConfig) error {
	if cfg.Email == "" || cfg.Password == "" {
		d.logger.Info("admin seeding skipped: ADMIN_EMAIL or ADMIN_PASSWORD not set")
		return nil
	}

	var existing models.User
	result := d.DB.Where("email = ?", cfg.Email).First(&existing)
	if result.Error == nil {
		// Admin exists — verify password matches env config and sync if needed
		if err := bcrypt.CompareHashAndPassword([]byte(existing.PasswordHash), []byte(cfg.Password)); err != nil {
			newHash, hashErr := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
			if hashErr != nil {
				return hashErr
			}
			existing.PasswordHash = string(newHash)
			existing.Role = "admin"
			existing.IsActive = true
			if updateErr := d.DB.Save(&existing).Error; updateErr != nil {
				d.logger.Error("failed to sync admin password", zap.Error(updateErr))
				return updateErr
			}
			d.logger.Info("admin password synced with env config", zap.String("email", sanitize.MaskEmail(cfg.Email)))
		} else {
			d.logger.Info("admin user already exists, password matches", zap.String("email", sanitize.MaskEmail(cfg.Email)))
		}
		// Back-fill org+project if missing (admin was created before this logic existed)
		d.ensureDefaultOrgProject(&existing)
		return nil
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		d.logger.Error("failed to hash admin password", zap.Error(err))
		return err
	}

	admin := &models.User{
		Email:                 cfg.Email,
		PasswordHash:          string(hashedPassword),
		Name:                  cfg.Name,
		Role:                  "admin",
		IsActive:              true,
		EmailVerified:         true,
		RequirePasswordChange: false,
	}

	if err := d.DB.Create(admin).Error; err != nil {
		d.logger.Error("failed to create admin user", zap.Error(err))
		return err
	}

	// Auto-create default org+project for new admin (same as register flow)
	d.ensureDefaultOrgProject(admin)

	d.logger.Info("default admin user created", zap.String("email", sanitize.MaskEmail(cfg.Email)))
	return nil
}

// ensureDefaultOrgProject creates a default Organization, Project, and membership
// for a user if they don't already belong to any organization.
func (d *Database) ensureDefaultOrgProject(user *models.User) {
	var count int64
	d.DB.Model(&models.OrganizationMember{}).Where("user_id = ?", user.ID).Count(&count)
	if count > 0 {
		return // user already has an org
	}

	orgName := "Default Org"
	if user.Name != "" {
		orgName = user.Name + "'s Org"
	}

	org := models.Organization{Name: orgName, OwnerID: user.ID}
	if err := d.DB.Create(&org).Error; err != nil {
		d.logger.Error("failed to create default org for admin", zap.Error(err))
		return
	}

	d.DB.Create(&models.OrganizationMember{OrgID: org.ID, UserID: user.ID, Role: "OWNER"})
	d.DB.Create(&models.Project{OrgID: org.ID, Name: "Default", Description: "Auto-created project"})

	// Grant welcome credit
	if user.Balance == 0 {
		user.Balance = 5.0
		d.DB.Model(user).UpdateColumn("balance", 5.0)
		d.DB.Create(&models.Transaction{OrgID: org.ID, UserID: user.ID, Type: "recharge", Amount: 5.0, Balance: 5.0, Description: "Welcome credit", Currency: "USD"})
	}

	d.logger.Info("created default org+project for user", zap.String("email", sanitize.MaskEmail(user.Email)), zap.String("orgId", org.ID.String()))
}

// CleanupOldHealthHistory removes health history records older than the specified duration.
// This should be called periodically to prevent unbounded table growth.
func (d *Database) CleanupOldHealthHistory(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := d.DB.Where("checked_at < ?", cutoff).Delete(&models.HealthHistory{})
	if result.Error != nil {
		d.logger.Error("failed to cleanup health history",
			zap.Int("retention_days", retentionDays),
			zap.Error(result.Error),
		)
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		d.logger.Info("cleaned up old health history records",
			zap.Int64("deleted", result.RowsAffected),
			zap.Int("retention_days", retentionDays),
		)
	}
	return result.RowsAffected, nil
}

// CleanupOldAlerts removes resolved alerts older than the specified duration.
func (d *Database) CleanupOldAlerts(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result := d.DB.Where("status = ? AND resolved_at < ?", "resolved", cutoff).Delete(&models.Alert{})
	if result.Error != nil {
		d.logger.Error("failed to cleanup old alerts",
			zap.Int("retention_days", retentionDays),
			zap.Error(result.Error),
		)
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		d.logger.Info("cleaned up old resolved alerts",
			zap.Int64("deleted", result.RowsAffected),
			zap.Int("retention_days", retentionDays),
		)
	}
	return result.RowsAffected, nil
}
