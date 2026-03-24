package config

import (
	"context"
	"encoding/json"
	"fmt"

	"llm-router-platform/internal/config"
	"llm-router-platform/internal/crypto"
	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"go.uber.org/zap"
)

type Service struct {
	repo   repository.ConfigRepo
	logger *zap.Logger
}

func NewService(repo repository.ConfigRepo, logger *zap.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) Get(ctx context.Context, key string) (string, error) {
	cfg, err := s.repo.Get(ctx, key)
	if err != nil {
		return "", err
	}

	if cfg.IsSecret {
		return crypto.Decrypt(cfg.Value)
	}
	return cfg.Value, nil
}

func (s *Service) Set(ctx context.Context, key, value, description, category string, isSecret bool) error {
	val := value
	if isSecret && val != "" {
		var err error
		val, err = crypto.Encrypt(val)
		if err != nil {
			return err
		}
	}

	cfg := &models.SystemConfig{
		Key:         key,
		Value:       val,
		Description: description,
		Category:    category,
		IsSecret:    isSecret,
	}
	return s.repo.Set(ctx, cfg)
}

func (s *Service) GetByCategory(ctx context.Context, category string) ([]models.SystemConfig, error) {
	return s.repo.GetByCategory(ctx, category)
}

// GetStripeConfig returns Stripe config from DB, falling back to env if not set.
func (s *Service) GetStripeConfig(ctx context.Context, env config.StripeConfig) config.StripeConfig {
	res := env

	if enabled, err := s.Get(ctx, "stripe_enabled"); err == nil {
		res.Enabled = enabled == "true"
	}
	if sk, err := s.Get(ctx, "stripe_secret_key"); err == nil && sk != "" {
		res.SecretKey = sk
	}
	if pk, err := s.Get(ctx, "stripe_publishable_key"); err == nil && pk != "" {
		res.PublishableKey = pk
	}
	if wh, err := s.Get(ctx, "stripe_webhook_secret"); err == nil && wh != "" {
		res.WebhookSecret = wh
	}

	return res
}

// ValidCategories lists the allowed settings categories.
var ValidCategories = map[string]bool{
	"site":        true,
	"security":    true,
	"defaults":    true,
	"email":       true,
	"backup":      true,
	"payment":     true,
	"oauth":       true,
	"featuregate": true,
}

// ─── Feature Gate persistence ───────────────────────────────────────

// InitFeatureGates loads all feature gate values from DB and merges them
// into the runtime FeatureGates struct. Called once during server startup
// after the database is ready.
func (s *Service) InitFeatureGates(fg *config.FeatureGates) {
	dbGates, err := s.LoadFeatureGates()
	if err != nil {
		s.logger.Warn("failed to load feature gates from DB, using defaults/env", zap.Error(err))
		return
	}
	if len(dbGates) > 0 {
		fg.MergeFromDB(dbGates)
		s.logger.Info("feature gates merged from database", zap.Int("count", len(dbGates)))
	}
}

// LoadFeatureGates reads all feature gate records from system_configs
// (category = "featuregate") and returns a map of field name -> bool.
func (s *Service) LoadFeatureGates() (map[string]bool, error) {
	configs, err := s.repo.GetByCategory(context.Background(), "featuregate")
	if err != nil {
		return nil, err
	}

	// Use a temporary FeatureGates to resolve DB keys back to field names
	tmp := &config.FeatureGates{}
	tmp.InitMeta()

	result := make(map[string]bool, len(configs))
	for _, c := range configs {
		fieldName := tmp.FieldNameFromDBKey(c.Key)
		if fieldName == "" {
			continue // skip stale or unknown keys
		}
		result[fieldName] = c.Value == "true"
	}
	return result, nil
}

// SetFeatureGate persists a single feature gate value to the database
// and updates the runtime FeatureGates struct.
func (s *Service) SetFeatureGate(fg *config.FeatureGates, name string, enabled bool) error {
	// Validate and update runtime (checks env override lock)
	if err := fg.Set(name, enabled); err != nil {
		return err
	}

	// Persist to DB
	dbKey := config.DBKey(name)
	val := "false"
	if enabled {
		val = "true"
	}

	gates := fg.ListGates()
	desc := ""
	for _, g := range gates {
		if g.Name == name {
			desc = g.Description
			break
		}
	}

	cfg := &models.SystemConfig{
		Key:         dbKey,
		Value:       val,
		Description: desc,
		Category:    "featuregate",
		IsSecret:    false,
	}
	if err := s.repo.Set(context.Background(), cfg); err != nil {
		s.logger.Error("failed to persist feature gate to DB",
			zap.String("gate", name),
			zap.Bool("enabled", enabled),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("feature gate updated",
		zap.String("gate", name),
		zap.Bool("enabled", enabled),
		zap.String("db_key", dbKey),
	)
	return nil
}

// GetAllSettings returns all settings grouped by category.
func (s *Service) GetAllSettings(ctx context.Context) (map[string]string, error) {
	configs, err := s.repo.GetByCategory(ctx, "settings")
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(configs))
	for _, c := range configs {
		// Strip "settings." prefix to get category name
		if len(c.Key) > 9 && c.Key[:9] == "settings." {
			result[c.Key[9:]] = c.Value
		}
	}
	return result, nil
}

// sensitiveFields defines which JSON keys contain secrets per category.
var sensitiveFields = map[string][]string{
	"email":   {"password"},
	"backup":  {"accessKey", "secretKey"},
	"payment": {"stripeSecretKey", "stripeWebhookSecret"},
	"oauth":   {"githubClientSecret", "googleClientSecret"},
}

// UpdateSettings writes settings JSON for a given category.
// Sensitive fields are encrypted before storage.
func (s *Service) UpdateSettings(ctx context.Context, category, data string) error {
	if !ValidCategories[category] {
		return fmt.Errorf("invalid settings category: %s", category)
	}

	value := data

	// Encrypt sensitive fields if any exist for this category
	if fields, ok := sensitiveFields[category]; ok {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(data), &parsed); err == nil {
			for _, field := range fields {
				if val, exists := parsed[field]; exists {
					if strVal, isStr := val.(string); isStr && strVal != "" {
						if encrypted, err := crypto.Encrypt(strVal); err == nil {
							parsed[field] = encrypted
						}
					}
				}
			}
			if out, err := json.Marshal(parsed); err == nil {
				value = string(out)
			}
		}
	}

	cfg := &models.SystemConfig{
		Key:         "settings." + category,
		Value:       value,
		Description: category + " settings",
		Category:    "settings",
		IsSecret:    len(sensitiveFields[category]) > 0,
	}
	return s.repo.Set(ctx, cfg)
}

// GetAllSettingsDecrypted returns all settings with sensitive fields decrypted.
func (s *Service) GetAllSettingsDecrypted(ctx context.Context) (map[string]string, error) {
	all, err := s.GetAllSettings(ctx)
	if err != nil {
		return nil, err
	}

	for category, jsonStr := range all {
		fields, ok := sensitiveFields[category]
		if !ok {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			continue
		}
		for _, field := range fields {
			if val, exists := parsed[field]; exists {
				if strVal, isStr := val.(string); isStr && strVal != "" {
					if decrypted, err := crypto.Decrypt(strVal); err == nil {
						parsed[field] = decrypted
					}
					// If decrypt fails, it might be plaintext — leave as-is
				}
			}
		}
		if out, err := json.Marshal(parsed); err == nil {
			all[category] = string(out)
		}
	}
	return all, nil
}

// GetPaymentStripeConfig returns the Stripe config from the payment settings JSON,
// falling back to the env-based config if not present.
func (s *Service) GetPaymentStripeConfig(ctx context.Context, env config.StripeConfig) config.StripeConfig {
	// First try the new JSON-based settings
	all, err := s.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return s.GetStripeConfig(ctx, env)
	}

	paymentJSON, ok := all["payment"]
	if !ok {
		return s.GetStripeConfig(ctx, env)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(paymentJSON), &parsed); err != nil {
		return s.GetStripeConfig(ctx, env)
	}

	res := env
	if v, ok := parsed["stripeEnabled"].(bool); ok {
		res.Enabled = v
	}
	if v, ok := parsed["stripeSecretKey"].(string); ok && v != "" {
		res.SecretKey = v
	}
	if v, ok := parsed["stripePublishableKey"].(string); ok && v != "" {
		res.PublishableKey = v
	}
	if v, ok := parsed["stripeWebhookSecret"].(string); ok && v != "" {
		res.WebhookSecret = v
	}
	return res
}

// GetOAuth2Config returns the OAuth2 config from DB settings, falling back to env.
func (s *Service) GetOAuth2Config(ctx context.Context, env config.OAuth2Config) config.OAuth2Config {
	all, err := s.GetAllSettingsDecrypted(ctx)
	if err != nil {
		return env
	}

	oauthJSON, ok := all["oauth"]
	if !ok {
		return env
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(oauthJSON), &parsed); err != nil {
		return env
	}

	res := env
	// GitHub
	if v, ok := parsed["githubEnabled"].(bool); ok && v {
		if id, ok := parsed["githubClientId"].(string); ok && id != "" {
			res.GitHub.ClientID = id
		}
		if secret, ok := parsed["githubClientSecret"].(string); ok && secret != "" {
			res.GitHub.ClientSecret = secret
		}
	} else if v, ok := parsed["githubEnabled"].(bool); ok && !v {
		// Explicitly disabled in DB → clear env fallback
		res.GitHub.ClientID = ""
		res.GitHub.ClientSecret = ""
	}
	// Google
	if v, ok := parsed["googleEnabled"].(bool); ok && v {
		if id, ok := parsed["googleClientId"].(string); ok && id != "" {
			res.Google.ClientID = id
		}
		if secret, ok := parsed["googleClientSecret"].(string); ok && secret != "" {
			res.Google.ClientSecret = secret
		}
	} else if v, ok := parsed["googleEnabled"].(bool); ok && !v {
		res.Google.ClientID = ""
		res.Google.ClientSecret = ""
	}
	return res
}
