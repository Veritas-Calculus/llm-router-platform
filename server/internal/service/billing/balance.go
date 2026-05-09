package billing

import (
	"context"
	"fmt"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"llm-router-platform/internal/service/email"
)

// BalanceService handles user credits and transactions.
type BalanceService struct {
	db       *gorm.DB
	userRepo repository.UserRepo
	txRepo   repository.TransactionRepo
	redis    *redis.Client
	emailSvc *email.Service
	logger   *zap.Logger
}

func NewBalanceService(
	db *gorm.DB,
	userRepo repository.UserRepo,
	txRepo repository.TransactionRepo,
	redisClient *redis.Client,
	emailSvc *email.Service,
	logger *zap.Logger,
) *BalanceService {
	return &BalanceService{
		db:       db,
		userRepo: userRepo,
		txRepo:   txRepo,
		redis:    redisClient,
		emailSvc: emailSvc,
		logger:   logger,
	}
}

// DeductBalance subtracts the cost of a request from the user's balance.
//
// The deduction and the matching transaction record are committed atomically
// under a row lock on the user. The low-balance alert is dispatched only
// AFTER the transaction commits — sending it inside the closure would still
// fire on rollback, producing false alerts. The lock acquisition is bounded
// by lockTimeout so a stuck peer cannot pin a request goroutine indefinitely.
func (s *BalanceService) DeductBalance(ctx context.Context, userID uuid.UUID, amount float64, description string, referenceID string) error {
	if amount <= 0 {
		return nil
	}

	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	var (
		alertEmail   string
		alertName    string
		alertBalance float64
		shouldAlert  bool
	)

	err := s.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance = roundCost(user.Balance - amount)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      roundCost(-amount),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		if user.Balance < 1.0 {
			alertEmail = user.Email
			alertName = user.Name
			alertBalance = user.Balance
			shouldAlert = true
		}
		return nil
	})
	if err != nil {
		if txCtx.Err() != nil {
			recordLockTimeout("deduct_balance")
		}
		billingDeductErrorsTotal.Inc()
		return err
	}

	if shouldAlert {
		s.sendLowBalanceAlert(ctx, userID, alertEmail, alertName, alertBalance)
	}
	return nil
}

// AddBalance adds credits to the user's balance (recharge or refund).
func (s *BalanceService) AddBalance(ctx context.Context, userID uuid.UUID, amount float64, txType string, description string, referenceID string) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	err := s.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance = roundCost(user.Balance + amount)
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        txType,
			Amount:      roundCost(amount),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		return tx.Create(transaction).Error
	})
	if err != nil && txCtx.Err() != nil {
		recordLockTimeout("add_balance")
	}
	return err
}

func (s *BalanceService) GetBalance(ctx context.Context, userID uuid.UUID) (float64, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}

func (s *BalanceService) GetTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Transaction, int64, error) {
	return s.txRepo.GetByUserID(ctx, userID, limit, offset)
}
