package billing

import (
	"context"
	"fmt"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// BalanceService handles user credits and transactions.
type BalanceService struct {
	db       *gorm.DB
	userRepo repository.UserRepo
	txRepo   repository.TransactionRepo
	logger   *zap.Logger
}

func NewBalanceService(
	db *gorm.DB,
	userRepo repository.UserRepo,
	txRepo repository.TransactionRepo,
	logger *zap.Logger,
) *BalanceService {
	return &BalanceService{
		db:       db,
		userRepo: userRepo,
		txRepo:   txRepo,
		logger:   logger,
	}
}

// DeductBalance subtracts the cost of a request from the user's balance.
func (s *BalanceService) DeductBalance(ctx context.Context, userID uuid.UUID, amount float64, description string, referenceID string) error {
	if amount <= 0 {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance -= amount
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			UserID:      userID,
			Type:        "deduction",
			Amount:      -amount,
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		return tx.Create(transaction).Error
	})
}

// AddBalance adds credits to the user's balance (recharge or refund).
func (s *BalanceService) AddBalance(ctx context.Context, userID uuid.UUID, amount float64, txType string, description string, referenceID string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance += amount
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			UserID:      userID,
			Type:        txType,
			Amount:      amount,
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		return tx.Create(transaction).Error
	})
}

func (s *BalanceService) GetTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Transaction, int64, error) {
	return s.txRepo.GetByUserID(ctx, userID, limit, offset)
}
