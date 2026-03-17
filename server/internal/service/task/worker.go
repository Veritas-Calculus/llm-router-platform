// Package task provides asynchronous task management and execution.
// This file implements the WorkerPool that polls for pending tasks
// and dispatches them to registered Executor implementations.
package task

import (
	"context"
	"sync"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Executor defines the interface for task execution.
// Each task type (e.g. "batch_tts", "batch_image") should implement this interface.
type Executor interface {
	// Execute runs the task and returns a result string or error.
	// The executor must call progressFn periodically to report progress (0-100).
	Execute(ctx context.Context, task *models.AsyncTask, progressFn func(int)) (result string, err error)
}

// WorkerPoolConfig holds configuration for the worker pool.
type WorkerPoolConfig struct {
	// Concurrency is the maximum number of tasks executing in parallel.
	Concurrency int
	// PollInterval is how often the pool checks for new pending tasks.
	PollInterval time.Duration
	// StaleTimeout is the maximum duration a task can be in "running" state
	// before being considered stale and re-queued.
	StaleTimeout time.Duration
}

// DefaultWorkerPoolConfig returns sensible defaults.
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		Concurrency:  4,
		PollInterval: 5 * time.Second,
		StaleTimeout: 30 * time.Minute,
	}
}

// WorkerPool polls the database for pending async tasks and dispatches
// them to registered Executor implementations via a bounded goroutine pool.
type WorkerPool struct {
	service   *Service
	db        *gorm.DB
	config    WorkerPoolConfig
	executors map[string]Executor // task type → executor
	sem       chan struct{}       // concurrency semaphore
	logger    *zap.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(service *Service, db *gorm.DB, cfg WorkerPoolConfig, logger *zap.Logger) *WorkerPool {
	return &WorkerPool{
		service:   service,
		db:        db,
		config:    cfg,
		executors: make(map[string]Executor),
		sem:       make(chan struct{}, cfg.Concurrency),
		logger:    logger.Named("task-worker"),
	}
}

// RegisterExecutor registers an executor for a task type.
func (wp *WorkerPool) RegisterExecutor(taskType string, executor Executor) {
	wp.executors[taskType] = executor
	wp.logger.Info("executor registered", zap.String("task_type", taskType))
}

// Start begins the polling loop. It blocks until ctx is cancelled.
func (wp *WorkerPool) Start(ctx context.Context) {
	ctx, wp.cancel = context.WithCancel(ctx)

	wp.logger.Info("worker pool starting",
		zap.Int("concurrency", wp.config.Concurrency),
		zap.Duration("poll_interval", wp.config.PollInterval),
	)

	ticker := time.NewTicker(wp.config.PollInterval)
	defer ticker.Stop()

	// Run an immediate poll on startup
	wp.pollAndDispatch(ctx)

	for {
		select {
		case <-ctx.Done():
			wp.logger.Info("worker pool stopping — waiting for in-flight tasks…")
			wp.wg.Wait()
			wp.logger.Info("worker pool stopped")
			return
		case <-ticker.C:
			wp.pollAndDispatch(ctx)
		}
	}
}

// Stop signals the worker pool to stop and waits for in-flight tasks.
func (wp *WorkerPool) Stop() {
	if wp.cancel != nil {
		wp.cancel()
	}
	wp.wg.Wait()
}

// pollAndDispatch fetches pending tasks from the database and dispatches them.
func (wp *WorkerPool) pollAndDispatch(ctx context.Context) {
	// Recover stale "running" tasks (e.g. from a crashed process)
	wp.recoverStaleTasks(ctx)

	// Fetch pending tasks, limited to available concurrency slots
	available := cap(wp.sem) - len(wp.sem)
	if available <= 0 {
		return // all worker slots are busy
	}

	tasks, err := wp.fetchPendingTasks(ctx, available)
	if err != nil {
		wp.logger.Error("failed to fetch pending tasks", zap.Error(err))
		return
	}

	for i := range tasks {
		task := tasks[i]

		// Check if we have an executor for this task type
		executor, ok := wp.executors[task.Type]
		if !ok {
			wp.logger.Warn("no executor registered for task type, skipping",
				zap.String("task_type", task.Type),
				zap.String("task_id", task.ID.String()),
			)
			continue
		}

		// Acquire semaphore slot (non-blocking since we limited fetch count)
		select {
		case wp.sem <- struct{}{}:
		default:
			return // no slot available — stop dispatching
		}

		wp.wg.Add(1)
		go wp.executeTask(ctx, &task, executor) // #nosec G118 -- ctx is the parent poll context //nolint:gosec
	}
}

// fetchPendingTasks atomically claims up to `limit` pending tasks by
// transitioning them to "running" status using SELECT … FOR UPDATE SKIP LOCKED.
func (wp *WorkerPool) fetchPendingTasks(ctx context.Context, limit int) ([]models.AsyncTask, error) {
	var tasks []models.AsyncTask

	err := wp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Find pending tasks and lock them
		if err := tx.Raw(`
			SELECT * FROM async_tasks
			WHERE status = 'pending' AND deleted_at IS NULL
			ORDER BY created_at ASC
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		`, limit).Scan(&tasks).Error; err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil
		}

		// Transition all fetched tasks to "running"
		ids := make([]uuid.UUID, len(tasks))
		for i := range tasks {
			ids[i] = tasks[i].ID
		}

		return tx.Model(&models.AsyncTask{}).
			Where("id IN ?", ids).
			Update("status", "running").Error
	})

	return tasks, err
}

// recoverStaleTasks resets tasks that have been "running" for too long
// (e.g. process crash) back to "pending" so they can be retried.
func (wp *WorkerPool) recoverStaleTasks(ctx context.Context) {
	staleThreshold := time.Now().Add(-wp.config.StaleTimeout)

	result := wp.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("status = ? AND updated_at < ? AND deleted_at IS NULL", "running", staleThreshold).
		Updates(map[string]interface{}{
			"status":   "pending",
			"progress": 0,
		})

	if result.Error != nil {
		wp.logger.Error("failed to recover stale tasks", zap.Error(result.Error))
		return
	}

	if result.RowsAffected > 0 {
		wp.logger.Warn("recovered stale tasks",
			zap.Int64("count", result.RowsAffected),
			zap.Duration("stale_timeout", wp.config.StaleTimeout),
		)
	}
}

// executeTask runs a single task via its executor and handles lifecycle transitions.
func (wp *WorkerPool) executeTask(ctx context.Context, task *models.AsyncTask, executor Executor) {
	defer wp.wg.Done()
	defer func() { <-wp.sem }() // release semaphore

	wp.logger.Info("executing task",
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", task.Type),
	)

	// Progress callback — updates DB with current progress
	progressFn := func(pct int) {
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		if err := wp.service.UpdateProgress(ctx, task.ID, pct); err != nil {
			wp.logger.Debug("failed to update progress",
				zap.String("task_id", task.ID.String()),
				zap.Error(err),
			)
		}
	}

	// Check if task was cancelled before execution
	freshTask, err := wp.service.GetTask(ctx, task.ID)
	if err != nil || freshTask.Status == "cancelled" {
		wp.logger.Info("task cancelled before execution",
			zap.String("task_id", task.ID.String()),
		)
		return
	}

	// Execute with a context that can be cancelled
	execCtx, execCancel := context.WithTimeout(ctx, wp.config.StaleTimeout)
	defer execCancel()

	result, execErr := executor.Execute(execCtx, task, progressFn)

	if execErr != nil {
		wp.logger.Error("task execution failed",
			zap.String("task_id", task.ID.String()),
			zap.String("task_type", task.Type),
			zap.Error(execErr),
		)
		_ = wp.service.FailTask(context.Background(), task.ID, execErr.Error()) //nolint:gosec // intentional: persist failure even if parent ctx cancelled
		return
	}

	wp.logger.Info("task completed",
		zap.String("task_id", task.ID.String()),
		zap.String("task_type", task.Type),
	)
	_ = wp.service.CompleteTask(context.Background(), task.ID, result) //nolint:gosec // intentional: persist result even if parent ctx cancelled
}
