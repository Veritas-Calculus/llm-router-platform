package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"llm-router-platform/internal/config"

	"go.uber.org/zap"
)

// LMStudioClient implements the Client interface for LM Studio (OpenAI-compatible).
type LMStudioClient struct {
	apiKey        string
	baseURL       string
	nativeBaseURL string
	httpClient    *http.Client
	logger        *zap.Logger
	loadedMu      sync.Mutex
	loadedModels  map[string]time.Time
}

const (
	localModelEnsureTTL = 30 * time.Minute
	localModelKeepAlive = "30m"
)

type lmStudioModelsResponse struct {
	Models []lmStudioModel `json:"models"`
}

type lmStudioModel struct {
	Type             string                   `json:"type"`
	Publisher        string                   `json:"publisher"`
	Key              string                   `json:"key"`
	DisplayName      string                   `json:"display_name"`
	Architecture     string                   `json:"architecture"`
	MaxContextLength int                      `json:"max_context_length"`
	Format           string                   `json:"format"`
	Description      string                   `json:"description"`
	Capabilities     json.RawMessage          `json:"capabilities"`
	LoadedInstances  []lmStudioLoadedInstance `json:"loaded_instances"`
}

type lmStudioLoadedInstance struct {
	ID     string `json:"id"`
	Config struct {
		ContextLength int `json:"context_length"`
	} `json:"config"`
}

// NewLMStudioClient creates a new LM Studio client.
func NewLMStudioClient(cfg *config.ProviderConfig, logger *zap.Logger) *LMStudioClient {
	httpClient := defaultHTTPClient()
	if cfg.HTTPClient != nil {
		httpClient = cfg.HTTPClient()
	}
	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")
	return &LMStudioClient{
		apiKey:        cfg.APIKey,
		baseURL:       baseURL,
		nativeBaseURL: lmStudioNativeBaseURL(baseURL),
		httpClient:    httpClient,
		logger:        logger,
		loadedModels:  make(map[string]time.Time),
	}
}

// Chat sends a chat completion request to LM Studio.
func (c *LMStudioClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
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
			Message:    "LM Studio chat completion error",
		}
	}

	var result ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Embeddings sends an embeddings request to LM Studio.
func (c *LMStudioClient) Embeddings(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	if req != nil {
		if err := c.ensureModelLoaded(ctx, req.Model); err != nil {
			return nil, err
		}
	}

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
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "LM Studio embeddings error",
		}
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, err
	}

	return &embResp, nil
}

// GenerateImage returns ErrNotImplemented.
func (c *LMStudioClient) GenerateImage(ctx context.Context, req *ImageGenerationRequest) (*ImageGenerationResponse, error) {
	return nil, ErrNotImplemented
}

// TranscribeAudio sends an audio transcription request to LM Studio's OpenAI-compatible endpoint.
func (c *LMStudioClient) TranscribeAudio(ctx context.Context, req *AudioTranscriptionRequest) (*AudioTranscriptionResponse, error) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", req.FileName)
	if err != nil {
		return nil, err
	}
	if _, err = fw.Write(req.File); err != nil {
		return nil, err
	}

	_ = w.WriteField("model", req.Model)
	if req.Language != "" {
		_ = w.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = w.WriteField("prompt", req.Prompt)
	}
	if req.ResponseFormat != "" {
		_ = w.WriteField("response_format", req.ResponseFormat)
	}
	if req.Temperature > 0 {
		_ = w.WriteField("temperature", fmt.Sprintf("%f", req.Temperature))
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &b)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", w.FormDataContentType())
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
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "LM Studio audio transcription error",
		}
	}

	if req.ResponseFormat == "text" || req.ResponseFormat == "srt" || req.ResponseFormat == "vtt" {
		respBody, _ := io.ReadAll(resp.Body)
		return &AudioTranscriptionResponse{Text: string(respBody)}, nil
	}

	var audioResp AudioTranscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&audioResp); err != nil {
		return nil, err
	}

	return &audioResp, nil
}

// SynthesizeSpeech sends a text-to-speech request to LM Studio's OpenAI-compatible endpoint.
func (c *LMStudioClient) SynthesizeSpeech(ctx context.Context, req *SpeechRequest) (*SpeechResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/speech", bytes.NewReader(body))
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
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "LM Studio speech synthesis error",
		}
	}

	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}

	return &SpeechResponse{
		Audio:       audioData,
		ContentType: contentType,
	}, nil
}

// ListModels returns available models from LM Studio.
func (c *LMStudioClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	nativeModels, supported, err := c.listNativeModels(ctx)
	if err == nil && supported {
		return lmStudioModelsToInfo(nativeModels), nil
	}
	if err != nil && c.logger != nil {
		c.logger.Debug("failed to list LM Studio native models", zap.Error(err))
	}

	return c.listOpenAIModels(ctx)
}

func (c *LMStudioClient) listOpenAIModels(ctx context.Context) ([]ModelInfo, error) {
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
		respBody, _ := io.ReadAll(resp.Body)
		return nil, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "failed to list LM Studio OpenAI-compatible models",
		}
	}

	var result struct {
		Data []ModelInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *LMStudioClient) listNativeModels(ctx context.Context) ([]lmStudioModel, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.nativeBaseURL+"/models", nil)
	if err != nil {
		return nil, false, err
	}
	c.addAuthHeader(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, true, &ProviderError{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
			Body:       respBody,
			Message:    "failed to list LM Studio native models",
		}
	}

	var result lmStudioModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, true, err
	}

	return result.Models, true, nil
}

func lmStudioModelsToInfo(models []lmStudioModel) []ModelInfo {
	out := make([]ModelInfo, 0, len(models))
	for _, mdl := range models {
		id := mdl.Key
		if id == "" {
			continue
		}

		name := mdl.DisplayName
		if name == "" {
			name = id
		}

		extra := map[string]json.RawMessage{}
		addExtra := func(key string, value interface{}) {
			raw, err := json.Marshal(value)
			if err == nil {
				extra[key] = raw
			}
		}

		addExtra("display_name", name)
		if mdl.Type != "" {
			addExtra("type", mdl.Type)
		}
		if mdl.Publisher != "" {
			addExtra("publisher", mdl.Publisher)
		}
		if mdl.Architecture != "" {
			addExtra("architecture", mdl.Architecture)
		}
		if mdl.Format != "" {
			addExtra("format", mdl.Format)
		}
		if mdl.Description != "" {
			addExtra("description", mdl.Description)
		}
		if mdl.MaxContextLength > 0 {
			addExtra("max_context_length", mdl.MaxContextLength)
			addExtra("max_tokens", mdl.MaxContextLength)
		}
		if len(mdl.Capabilities) > 0 {
			extra["capabilities"] = mdl.Capabilities
		}
		addExtra("loaded", len(mdl.LoadedInstances) > 0)

		out = append(out, ModelInfo{
			ID:    id,
			Name:  name,
			Extra: extra,
		})
	}
	return out
}

// CheckHealth verifies LM Studio is accessible.
func (c *LMStudioClient) CheckHealth(ctx context.Context) (bool, time.Duration, error) {
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

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return false, latency, errors.New("API returned HTTP " + resp.Status + ": " + string(respBody))
	}

	return true, latency, nil
}

// StreamChat sends a streaming chat completion request to LM Studio.
func (c *LMStudioClient) StreamChat(ctx context.Context, req *ChatRequest) (<-chan StreamChunk, error) {
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
			Message:    "LM Studio streaming chat completion error",
		}
	}

	chunks := make(chan StreamChunk)
	go processSSEStream(ctx, resp.Body, chunks, c.logger)

	return chunks, nil
}

func lmStudioNativeBaseURL(baseURL string) string {
	root := strings.TrimSuffix(baseURL, "/")
	root = strings.TrimSuffix(root, "/v1")
	return root + "/api/v1"
}

func (c *LMStudioClient) ensureModelLoaded(ctx context.Context, model string) error {
	model = strings.TrimSpace(model)
	if model == "" || c.isModelRecentlyEnsured(model) {
		return nil
	}

	models, supported, err := c.listNativeModels(ctx)
	if !supported {
		return nil
	}
	if err == nil {
		for _, mdl := range models {
			if mdl.matches(model) && len(mdl.LoadedInstances) > 0 {
				c.markModelEnsured(model)
				return nil
			}
		}
	} else if c.logger != nil {
		c.logger.Debug("failed to inspect LM Studio loaded models before load", zap.String("model", model), zap.Error(err))
	}

	if err := c.loadModel(ctx, model); err != nil {
		return err
	}
	c.markModelEnsured(model)
	return nil
}

func (m lmStudioModel) matches(model string) bool {
	if m.Key == model {
		return true
	}
	for _, instance := range m.LoadedInstances {
		if instance.ID == model {
			return true
		}
	}
	return false
}

func (c *LMStudioClient) loadModel(ctx context.Context, model string) error {
	payload := map[string]interface{}{
		"model":            model,
		"echo_load_config": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.nativeBaseURL+"/models/load", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	c.addAuthHeader(httpReq)

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
			Message:    "LM Studio model load error",
		}
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *LMStudioClient) isModelRecentlyEnsured(model string) bool {
	c.loadedMu.Lock()
	defer c.loadedMu.Unlock()

	loadedAt, ok := c.loadedModels[model]
	return ok && time.Since(loadedAt) < localModelEnsureTTL
}

func (c *LMStudioClient) markModelEnsured(model string) {
	c.loadedMu.Lock()
	defer c.loadedMu.Unlock()

	c.loadedModels[model] = time.Now()
}

func (c *LMStudioClient) addAuthHeader(req *http.Request) {
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}
