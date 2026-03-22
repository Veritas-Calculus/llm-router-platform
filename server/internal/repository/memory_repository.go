package repository

import (
	"context"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ConversationMemoryRepository handles conversation memory data access.
type ConversationMemoryRepository struct {
	db *gorm.DB
}

// NewConversationMemoryRepository creates a new conversation memory repository.
func NewConversationMemoryRepository(db *gorm.DB) *ConversationMemoryRepository {
	return &ConversationMemoryRepository{db: db}
}

// Create inserts a new conversation memory.
func (r *ConversationMemoryRepository) Create(ctx context.Context, memory *models.ConversationMemory) error {
	return r.db.WithContext(ctx).Create(memory).Error
}

// GetByConversation retrieves messages for a conversation.
func (r *ConversationMemoryRepository) GetByConversation(ctx context.Context, projectID uuid.UUID, conversationID string) ([]models.ConversationMemory, error) {
	var memories []models.ConversationMemory
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND conversation_id = ?", projectID, conversationID).
		Order("sequence ASC").
		Find(&memories).Error; err != nil {
		return nil, err
	}
	return memories, nil
}

// DeleteByConversation permanently removes all messages in a conversation.
func (r *ConversationMemoryRepository) DeleteByConversation(ctx context.Context, projectID uuid.UUID, conversationID string) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("project_id = ? AND conversation_id = ?", projectID, conversationID).
		Delete(&models.ConversationMemory{}).Error
}

// DeleteOldestByConversation deletes the oldest N messages from a conversation.
func (r *ConversationMemoryRepository) DeleteOldestByConversation(ctx context.Context, projectID uuid.UUID, conversationID string, count int) error {
	// Find the oldest N message IDs
	var ids []uuid.UUID
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMemory{}).
		Where("project_id = ? AND conversation_id = ?", projectID, conversationID).
		Order("sequence ASC").
		Limit(count).
		Pluck("id", &ids).Error; err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Unscoped().
		Where("id IN ?", ids).
		Delete(&models.ConversationMemory{}).Error
}

// ListConversationIDs returns all conversation IDs for a user.
func (r *ConversationMemoryRepository) ListConversationIDs(ctx context.Context, projectID uuid.UUID) ([]string, error) {
	var ids []string
	if err := r.db.WithContext(ctx).
		Model(&models.ConversationMemory{}).
		Where("project_id = ?", projectID).
		Distinct("conversation_id").
		Pluck("conversation_id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
