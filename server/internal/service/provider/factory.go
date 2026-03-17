package provider

import (
	"fmt"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// NewClientByName creates a provider Client by provider name.
// This is the single source of truth for mapping provider names to client
// constructors, eliminating the duplicated switch blocks in Router and Health.
// Every client is wrapped with RetryClient for automatic transient error retry.
func NewClientByName(name string, cfg *config.ProviderConfig, logger *zap.Logger) (Client, error) {
	return NewClientByNameWithRetry(name, cfg, DefaultRetryConfig(), logger)
}

// NewClientByNameWithRetry creates a provider Client with a custom retry config.
// Use RetryConfigFromProvider(maxRetries, timeout) to build from models.Provider.
func NewClientByNameWithRetry(name string, cfg *config.ProviderConfig, retryCfg RetryConfig, logger *zap.Logger) (Client, error) {
	var inner Client

	switch name {
	case "openai":
		inner = NewOpenAIClient(cfg, logger)
	case "anthropic":
		inner = NewAnthropicClient(cfg, logger)
	case "google":
		inner = NewGoogleClient(cfg, logger)
	case "ollama":
		inner = NewOllamaClient(cfg, logger)
	case "lmstudio":
		inner = NewLMStudioClient(cfg, logger)
	case "deepseek":
		inner = NewDeepSeekClient(cfg, logger)
	case "mistral":
		inner = NewMistralClient(cfg, logger)
	case "vllm":
		inner = NewOpenAIClient(cfg, logger)
	default:
		// Fall back to OpenAI-compatible client for unknown providers
		logger.Debug("unknown provider name, falling back to OpenAI-compatible client",
			zap.String("provider", name))
		inner = NewOpenAIClient(cfg, logger)
	}

	// Wrap with retry decorator for automatic transient error handling.
	// Retry layer handles: connection refused, timeouts, 5xx errors.
	// Key rotation (quota/rate-limit) is handled separately by Router.
	return NewRetryClient(inner, retryCfg, logger), nil
}

// MustNewClientByName is like NewClientByName but panics on error.
// Useful during initialization where failure is unrecoverable.
func MustNewClientByName(name string, cfg *config.ProviderConfig, logger *zap.Logger) Client {
	client, err := NewClientByName(name, cfg, logger)
	if err != nil {
		panic(fmt.Sprintf("failed to create provider client %q: %v", name, err))
	}
	return client
}
