package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// OllamaClient implements the Client interface for Ollama (OpenAI-compatible).
type OllamaClient struct {
	apiKey        string
	baseURL       string
	nativeBaseURL string
	httpClient    *http.Client
	logger        *zap.Logger
	loadedMu      sync.Mutex
	loadedModels  map[string]time.Time
}

// NewOllamaClient creates a new Ollama client.
func NewOllamaClient(cfg *config.ProviderConfig, logger *zap.Logger) *OllamaClient {
	httpClient := &http.Client{
		Timeout: 600 * time.Second,
	}
	if cfg.HTTPClient != nil {
		httpClient = cfg.HTTPClient()
	}

	inputBaseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	nativeBaseURL := ollamaNativeBaseURL(inputBaseURL)

	// Ensure baseURL ends with /v1 for OpenAI compatibility if not already present.
	baseURL := inputBaseURL
	if !strings.HasSuffix(baseURL, "/v1") && !strings.Contains(baseURL, "/v1/") {
		baseURL = strings.TrimSuffix(baseURL, "/") + "/v1"
	}

	return &OllamaClient{
		apiKey:        cfg.APIKey,
		baseURL:       baseURL,
		nativeBaseURL: nativeBaseURL,
		httpClient:    httpClient,
		logger:        logger,
		loadedModels:  make(map[string]time.Time),
	}
}

// Chat sends a chat completion request to Ollama.
func (c *OllamaClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	if req != nil {
		if err := c.ensureModelLoaded(ctx, req.Model); err != nil {
			return nil, err
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       bodyBytes,
			Message:    "Ollama chat completion error",
		}
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Embeddings sends an embeddings request to Ollama.
func (c *OllamaClient) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, errors.New(string(respBody))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, err
	}

	return &embResp, nil
}

// GenerateImage returns ErrNotImplemented.
func (c *OllamaClient) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	return nil, ErrNotImplemented
}

// TranscribeAudio returns ErrNotImplemented.
func (c *OllamaClient) TranscribeAudio(_ context.Context, _ *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	return nil, ErrNotImplemented
}

// SynthesizeSpeech returns ErrNotImplemented.
func (c *OllamaClient) SynthesizeSpeech(_ context.Context, _ *SpeechRequest) (*SpeechResponse, error) {
	return nil, ErrNotImplemented
}

// ListModels returns available models from Ollama.
func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to list models")
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

// CheckHealth verifies Ollama is accessible.
func (c *OllamaClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
	start := time.Now()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return false, 0, err
	}

	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	latency := time.Since(start)
	if err != nil {
		return false, latency, err
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, latency, nil
}

// StreamChat sends a streaming chat completion request to Ollama.
func (c *OllamaClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
	if req != nil {
		if err := c.ensureModelLoaded(ctx, req.Model); err != nil {
			return nil, err
		}
	}

	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "Ollama streaming chat completion error",
		}
	}

	chunks := make(chan StreamChunk)
	go processSSEStream(ctx, resp.Body, chunks, c.logger)

	return chunks, nil
}

func ollamaNativeBaseURL(baseURL string) string {
	root := strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(root, "/v1") {
		root = strings.TrimSuffix(root, "/v1")
	}
	return root
}

func (c *OllamaClient) ensureModelLoaded(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" || c.isModelRecentlyEnsured(model) {
		return nil
	}

	payload := map[string]interface{}{
		"model":      model,
		"prompt":     "",
		"stream":     false,
		"keep_alive": localModelKeepAlive,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.nativeBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(resp.Body)
		return &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "Ollama model preload error",
		}
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	c.markModelEnsured(model)
	return nil
}

func (c *OllamaClient) isModelRecentlyEnsured(model string) bool {
	c.loadedMu.Lock()
	defer c.loadedMu.Unlock()

	loadedAt, ok := c.loadedModels[model]
	return ok && time.Since(loadedAt) < localModelEnsureTTL
}

func (c *OllamaClient) markModelEnsured(model string) {
	c.loadedMu.Lock()
	defer c.loadedMu.Unlock()

	c.loadedModels[model] = time.Now()
}
