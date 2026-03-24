package billing

import (
	"context"
	"fmt"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

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
func (s *BalanceService) DeductBalance(ctx context.Context, userID uuid.UUID, amount float64, description string, referenceID string) error {
	if amount <= 0 {
		return nil
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance -= amount
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      -amount,
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		err := tx.Create(transaction).Error
		if err != nil {
			return err
		}

		// Check for low balance alert (threshold $1.00)
		if s.redis != nil && s.emailSvc != nil && user.Balance < 1.0 {
			cacheKey := fmt.Sprintf("quota_warn:balance:%s", userID.String())
			if err := s.redis.Get(ctx, cacheKey).Err(); err == redis.Nil { // Not warned yet
				s.logger.Info("sending low balance warning email", zap.String("userID", userID.String()), zap.Float64("balance", user.Balance))

				// Send warning asynchronously
				go func(to, name string, currentBalance float64) {
					if err := s.emailSvc.SendQuotaWarningEmail(to, name, fmt.Sprintf("$%.2f", currentBalance), "$1.00"); err != nil {
						s.logger.Error("failed to send quota warning email", zap.Error(err))
					}
				}(user.Email, user.Name, user.Balance)

				// Cache the warning state for 24 hours to prevent spam
				s.redis.Set(ctx, cacheKey, "1", 24*time.Hour)
			}
		}

		return nil
	})
	if err != nil {
		billingDeductErrorsTotal.Inc()
	}
	return err
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
			OrgID:       userID,
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
