package provider

import (
	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// DeepSeekClient implements the Client interface for DeepSeek.
// DeepSeek provides an OpenAI-compatible API, so we reuse OpenAIClient
// with a different base URL default.
type DeepSeekClient struct {
	*OpenAIClient
}

// NewDeepSeekClient creates a new DeepSeek client.
func NewDeepSeekClient(cfg *config.ProviderConfig, logger *zap.Logger) *DeepSeekClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com"
	}
	return &DeepSeekClient{
		OpenAIClient: NewOpenAIClient(cfg, logger),
	}
}
