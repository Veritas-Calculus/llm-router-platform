package config

import (
	"context"

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
