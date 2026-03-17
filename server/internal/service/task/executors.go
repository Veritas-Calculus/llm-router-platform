package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"llm-router-platform/internal/models"

	"go.uber.org/zap"
)

// ─── Placeholder Executors ─────────────────────────────────────────
// These stub executors demonstrate the Executor interface contract.
// Replace or extend with real implementations that call Provider clients.

// BatchTTSInput represents the input for batch TTS tasks.
type BatchTTSInput struct {
	Items []BatchTTSItem `json:"items"`
}

// BatchTTSItem is a single text-to-speech item.
type BatchTTSItem struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Model string `json:"model,omitempty"`
}

// BatchTTSExecutor processes batch text-to-speech tasks.
type BatchTTSExecutor struct {
	logger *zap.Logger
}

// NewBatchTTSExecutor creates a new batch TTS executor.
func NewBatchTTSExecutor(logger *zap.Logger) *BatchTTSExecutor {
	return &BatchTTSExecutor{logger: logger.Named("batch-tts")}
}

// Execute processes a batch TTS task.
func (e *BatchTTSExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input BatchTTSInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if len(input.Items) == 0 {
		return "", fmt.Errorf("no items to process")
	}

	results := make([]string, 0, len(input.Items))
	for i, item := range input.Items {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		e.logger.Debug("processing TTS item",
			zap.Int("index", i),
			zap.String("text_preview", truncate(item.Text, 50)),
		)

		// TODO: Call actual TTS provider here via router
		// For now, record that the item was processed
		results = append(results, fmt.Sprintf("item_%d: processed (%d chars)", i, len(item.Text)))

		// Report progress
		pct := int(float64(i+1) / float64(len(input.Items)) * 100)
		progressFn(pct)

		// Simulate processing time (remove when real provider is wired)
		time.Sleep(100 * time.Millisecond)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"processed": len(results),
		"items":     results,
	})
	return string(resultJSON), nil
}

// ─── Batch Image Executor ──────────────────────────────────────────

// BatchImageInput represents the input for batch image generation tasks.
type BatchImageInput struct {
	Prompts []string `json:"prompts"`
	Model   string   `json:"model,omitempty"`
	Size    string   `json:"size,omitempty"`
}

// BatchImageExecutor processes batch image generation tasks.
type BatchImageExecutor struct {
	logger *zap.Logger
}

// NewBatchImageExecutor creates a new batch image executor.
func NewBatchImageExecutor(logger *zap.Logger) *BatchImageExecutor {
	return &BatchImageExecutor{logger: logger.Named("batch-image")}
}

// Execute processes a batch image generation task.
func (e *BatchImageExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input BatchImageInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if len(input.Prompts) == 0 {
		return "", fmt.Errorf("no prompts to process")
	}

	results := make([]string, 0, len(input.Prompts))
	for i, prompt := range input.Prompts {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		e.logger.Debug("processing image prompt",
			zap.Int("index", i),
			zap.String("prompt_preview", truncate(prompt, 50)),
		)

		// TODO: Call actual image generation provider here via router
		results = append(results, fmt.Sprintf("image_%d: generated", i))

		pct := int(float64(i+1) / float64(len(input.Prompts)) * 100)
		progressFn(pct)

		time.Sleep(100 * time.Millisecond)
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"processed": len(results),
		"images":    results,
	})
	return string(resultJSON), nil
}

// ─── Video Analysis Executor ───────────────────────────────────────

// VideoAnalysisInput represents the input for video analysis tasks.
type VideoAnalysisInput struct {
	VideoURL string `json:"video_url"`
	Model    string `json:"model,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
}

// VideoAnalysisExecutor processes video analysis tasks.
type VideoAnalysisExecutor struct {
	logger *zap.Logger
}

// NewVideoAnalysisExecutor creates a new video analysis executor.
func NewVideoAnalysisExecutor(logger *zap.Logger) *VideoAnalysisExecutor {
	return &VideoAnalysisExecutor{logger: logger.Named("video-analysis")}
}

// Execute processes a video analysis task.
func (e *VideoAnalysisExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input VideoAnalysisInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.VideoURL == "" {
		return "", fmt.Errorf("video_url is required")
	}

	e.logger.Debug("processing video analysis",
		zap.String("video_url", input.VideoURL),
	)

	progressFn(10)

	// TODO: Call actual video analysis provider (e.g. Gemini 2.x) here
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(200 * time.Millisecond): // simulated processing
	}

	progressFn(90)

	result, _ := json.Marshal(map[string]interface{}{
		"video_url": input.VideoURL,
		"analysis":  "Video analysis placeholder — wire actual provider for real results",
	})

	progressFn(100)
	return string(result), nil
}

// ─── Single TTS Executor ───────────────────────────────────────────

// TTSInput represents the input for a single TTS task.
type TTSInput struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Model string `json:"model,omitempty"`
}

// TTSExecutor processes single text-to-speech tasks.
type TTSExecutor struct {
	logger *zap.Logger
}

// NewTTSExecutor creates a new TTS executor.
func NewTTSExecutor(logger *zap.Logger) *TTSExecutor {
	return &TTSExecutor{logger: logger.Named("tts")}
}

// Execute processes a single TTS task.
func (e *TTSExecutor) Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (string, error) {
	var input TTSInput
	if err := json.Unmarshal([]byte(task.Input), &input); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if input.Text == "" {
		return "", fmt.Errorf("text is required")
	}

	e.logger.Debug("processing TTS",
		zap.String("text_preview", truncate(input.Text, 50)),
	)

	progressFn(30)

	// TODO: Call actual TTS provider here
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}

	progressFn(100)

	result, _ := json.Marshal(map[string]interface{}{
		"text_length": len(input.Text),
		"status":      "synthesized",
	})
	return string(result), nil
}

// ─── Helpers ───────────────────────────────────────────────────────

// truncate shortens a string to maxLen characters with ellipsis.
func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// RegisterDefaultExecutors registers all built-in task executors on the pool.
func RegisterDefaultExecutors(pool *WorkerPool, logger *zap.Logger) {
	pool.RegisterExecutor("tts", NewTTSExecutor(logger))
	pool.RegisterExecutor("batch_tts", NewBatchTTSExecutor(logger))
	pool.RegisterExecutor("batch_image", NewBatchImageExecutor(logger))
	pool.RegisterExecutor("video_analysis", NewVideoAnalysisExecutor(logger))
}
