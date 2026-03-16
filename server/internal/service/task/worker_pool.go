package task

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// TaskFunc is a function that processes a task. It receives the task ID.
type TaskFunc func(ctx context.Context, taskID string) error

// WorkerPool manages a pool of goroutines for processing async tasks.
type WorkerPool struct {
	workers    int
	jobCh      chan job
	wg         sync.WaitGroup
	cancelFunc context.CancelFunc
	logger     *zap.Logger
}

type job struct {
	taskID string
	fn     TaskFunc
}

// NewWorkerPool creates a worker pool with the given concurrency.
// Workers start immediately and drain gracefully on Stop().
func NewWorkerPool(workers int, queueSize int, logger *zap.Logger) *WorkerPool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	return &WorkerPool{
		workers: workers,
		jobCh:   make(chan job, queueSize),
		logger:  logger,
	}
}

// Start launches workers that pull from the job channel.
func (wp *WorkerPool) Start(ctx context.Context) {
	ctx, wp.cancelFunc = context.WithCancel(ctx)

	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}

	wp.logger.Info("task worker pool started",
		zap.Int("workers", wp.workers),
		zap.Int("queue_size", cap(wp.jobCh)),
	)
}

// Submit enqueues a task for processing. Returns false if the queue is full.
func (wp *WorkerPool) Submit(taskID string, fn TaskFunc) bool {
	select {
	case wp.jobCh <- job{taskID: taskID, fn: fn}:
		return true
	default:
		wp.logger.Warn("worker pool queue full, task rejected",
			zap.String("task_id", taskID),
		)
		return false
	}
}

// Stop signals all workers to stop and waits for in-flight tasks to finish.
func (wp *WorkerPool) Stop() {
	if wp.cancelFunc != nil {
		wp.cancelFunc()
	}
	close(wp.jobCh)
	wp.wg.Wait()
	wp.logger.Info("task worker pool stopped")
}

// Pending returns the number of jobs waiting in the queue.
func (wp *WorkerPool) Pending() int {
	return len(wp.jobCh)
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	for j := range wp.jobCh {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := j.fn(ctx, j.taskID); err != nil {
			wp.logger.Error("task execution failed",
				zap.Int("worker", id),
				zap.String("task_id", j.taskID),
				zap.Error(err),
			)
		} else {
			wp.logger.Debug("task completed",
				zap.Int("worker", id),
				zap.String("task_id", j.taskID),
			)
		}
	}
}
