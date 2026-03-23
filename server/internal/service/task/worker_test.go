package task

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─── Test Helpers ──────────────────────────────────────────────────

var testLogger, _ = zap.NewDevelopment()

func noopProgress(_ int) {}

func testAsyncTask(taskType, input string) *models.AsyncTask {
	return &models.AsyncTask{
		BaseModel: models.BaseModel{ID: uuid.New()},
		ProjectID: uuid.New(),
		Type:      taskType,
		Status:    "pending",
		Input:     input,
	}
}

// ─── Executor Unit Tests ───────────────────────────────────────────
// These tests validate input parsing and validation only.
// Actual provider calls require a running router, tested via integration tests.

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello…"},
		{"hello\nworld", 20, "hello world"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestBatchTTSExecutor_InvalidInput(t *testing.T) {
	exec := NewBatchTTSExecutor(nil, testLogger)
	task := testAsyncTask("batch_tts", "invalid json")
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for invalid JSON input")
	}
}

func TestBatchTTSExecutor_EmptyItems(t *testing.T) {
	exec := NewBatchTTSExecutor(nil, testLogger)
	input, _ := json.Marshal(BatchTTSInput{Items: []BatchTTSItem{}})
	task := testAsyncTask("batch_tts", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for empty items")
	}
}

func TestBatchImageExecutor_InvalidInput(t *testing.T) {
	exec := NewBatchImageExecutor(nil, testLogger)
	task := testAsyncTask("batch_image", "bad")
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestBatchImageExecutor_EmptyPrompts(t *testing.T) {
	exec := NewBatchImageExecutor(nil, testLogger)
	input, _ := json.Marshal(BatchImageInput{Prompts: []string{}})
	task := testAsyncTask("batch_image", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for empty prompts")
	}
}

func TestVideoAnalysisExecutor_MissingURL(t *testing.T) {
	exec := NewVideoAnalysisExecutor(nil, testLogger)
	input, _ := json.Marshal(VideoAnalysisInput{VideoURL: ""})
	task := testAsyncTask("video_analysis", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for missing video_url")
	}
}

func TestTTSExecutor_EmptyText(t *testing.T) {
	exec := NewTTSExecutor(nil, testLogger)
	input, _ := json.Marshal(TTSInput{Text: ""})
	task := testAsyncTask("tts", string(input))
	_, err := exec.Execute(context.Background(), task, noopProgress)
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	// BatchTTS with nil router: Route() will panic, but context should cancel before that
	// Use a very short timeout to ensure context cancellation is checked first
	exec := NewBatchTTSExecutor(nil, testLogger)
	items := make([]BatchTTSItem, 100)
	for i := range items {
		items[i] = BatchTTSItem{Text: "test"}
	}
	input, _ := json.Marshal(BatchTTSInput{Items: items})
	task := testAsyncTask("batch_tts", string(input))

	// Use an already-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := exec.Execute(ctx, task, noopProgress)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

// ─── WorkerPool Config Tests ──────────────────────────────────────

func TestDefaultWorkerPoolConfig(t *testing.T) {
	cfg := DefaultWorkerPoolConfig()
	if cfg.Concurrency != 4 {
		t.Errorf("expected concurrency 4, got %d", cfg.Concurrency)
	}
	if cfg.PollInterval != 5*time.Second {
		t.Errorf("expected poll interval 5s, got %v", cfg.PollInterval)
	}
	if cfg.StaleTimeout != 30*time.Minute {
		t.Errorf("expected stale timeout 30m, got %v", cfg.StaleTimeout)
	}
}

func TestRegisterDefaultExecutors(t *testing.T) {
	pool := NewWorkerPool(nil, nil, DefaultWorkerPoolConfig(), testLogger)
	RegisterDefaultExecutors(pool, nil, testLogger)

	expectedTypes := []string{"tts", "batch_tts", "batch_image", "video_analysis"}
	for _, typ := range expectedTypes {
		if _, ok := pool.executors[typ]; !ok {
			t.Errorf("expected executor for type %q to be registered", typ)
		}
	}
}
