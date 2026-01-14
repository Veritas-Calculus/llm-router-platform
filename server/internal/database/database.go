// Package database provides database connection and management.
package database

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"llm-router-platform/internal/config"
	"llm-router-platform/internal/models"
)

// Database wraps the GORM database connection.
type Database struct {
	DB     *gorm.DB
	logger *zap.Logger
}

// New creates a new database connection.
func New(cfg *config.DatabaseConfig, log *zap.Logger) (*Database, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), gormConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &Database{
		DB:     db,
		logger: log,
	}, nil
}

// Migrate runs database migrations.
func (d *Database) Migrate() error {
	return d.DB.AutoMigrate(
		&models.User{},
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
	)
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
			Name:       "openai",
			BaseURL:    "https://api.openai.com/v1",
			IsActive:   true,
			Priority:   10,
			Weight:     1.0,
			MaxRetries: 3,
			Timeout:    30,
		},
		{
			Name:       "anthropic",
			BaseURL:    "https://api.anthropic.com",
			IsActive:   true,
			Priority:   9,
			Weight:     1.0,
			MaxRetries: 3,
			Timeout:    30,
		},
		{
			Name:       "google",
			BaseURL:    "https://generativelanguage.googleapis.com",
			IsActive:   false,
			Priority:   8,
			Weight:     1.0,
			MaxRetries: 3,
			Timeout:    30,
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
