package repository

import (
	"context"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TaskRepository handles async task data access.
type TaskRepository struct {
	db *gorm.DB
}

// NewTaskRepository creates a new task repository.
func NewTaskRepository(db *gorm.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create inserts a new async task.
func (r *TaskRepository) Create(ctx context.Context, task *models.AsyncTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

// GetByID retrieves a task by its ID.
func (r *TaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AsyncTask, error) {
	var task models.AsyncTask
	if err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// ListByProjectID returns tasks for a user with optional status filter and pagination.
func (r *TaskRepository) ListByProjectID(ctx context.Context, projectID uuid.UUID, status string, limit, offset int) ([]models.AsyncTask, int64, error) {
	var tasks []models.AsyncTask
	var total int64

	query := r.db.WithContext(ctx).Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Model(&models.AsyncTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// UpdateProgress sets a task's progress percentage.
func (r *TaskRepository) UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error {
	return r.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ?", id).
		Update("progress", progress).Error
}

// UpdateStatus sets task status and related fields atomically.
func (r *TaskRepository) UpdateStatus(ctx context.Context, id uuid.UUID, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// CancelByID marks a pending or running task as cancelled.
func (r *TaskRepository) CancelByID(ctx context.Context, id uuid.UUID, completedAt *time.Time) error {
	return r.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("id = ? AND status IN ?", id, []string{"pending", "running"}).
		Updates(map[string]interface{}{
			"status":       "cancelled",
			"completed_at": completedAt,
		}).Error
}

// ClaimPending atomically claims up to `limit` pending tasks using SELECT FOR UPDATE SKIP LOCKED,
// transitioning them to "running" status.
func (r *TaskRepository) ClaimPending(ctx context.Context, limit int) ([]models.AsyncTask, error) {
	var tasks []models.AsyncTask

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "SKIP LOCKED",
		}).Where("status = ? AND deleted_at IS NULL", "pending").
			Order("created_at ASC").
			Limit(limit).
			Find(&tasks).Error; err != nil {
			return err
		}

		if len(tasks) == 0 {
			return nil
		}

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

// RecoverStale resets tasks stuck in "running" longer than the threshold back to "pending".
// Returns the number of recovered tasks.
func (r *TaskRepository) RecoverStale(ctx context.Context, staleThreshold time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&models.AsyncTask{}).
		Where("status = ? AND updated_at < ? AND deleted_at IS NULL", "running", staleThreshold).
		Updates(map[string]interface{}{
			"status":   "pending",
			"progress": 0,
		})
	return result.RowsAffected, result.Error
}
