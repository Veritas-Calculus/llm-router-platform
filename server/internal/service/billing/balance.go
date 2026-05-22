package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"llm-router-platform/internal/service/email"
)

// ErrInsufficientBalance is returned by DeductBalance when the user does not
// have enough credit to cover the requested amount. Callers should gate
// requests on this and return a 402 Payment Required (or equivalent) so the
// DB-level CHECK (balance >= 0) constraint never aborts a billing transaction.
var ErrInsufficientBalance = errors.New("insufficient balance")

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

		// Refuse to overdraft so the new CHECK (balance >= 0) constraint
		// never aborts a billing transaction mid-write. Callers should
		// gate on GetBalance / quotas; this is the last-line guard.
		newBalance := roundCost(user.Balance - amount)
		if newBalance < 0 {
			return ErrInsufficientBalance
		}
		user.Balance = newBalance
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
	return s.addBalance(ctx, userID, amount, txType, description, referenceID, "")
}

// AddBalanceIdempotent adds credits and attaches an idempotency key. A second
// call with the same key — typically the upstream provider's event/transaction
// id, e.g. Stripe evt_…, WeChat transaction_id, Alipay trade_no — is rejected
// by the partial unique index on transactions(idempotency_key) and surfaces as
// a no-op (returns nil so the webhook can ack the duplicate retry).
func (s *BalanceService) AddBalanceIdempotent(ctx context.Context, userID uuid.UUID, amount float64, txType, description, referenceID, idempotencyKey string) error {
	return s.addBalance(ctx, userID, amount, txType, description, referenceID, idempotencyKey)
}

func (s *BalanceService) addBalance(ctx context.Context, userID uuid.UUID, amount float64, txType, description, referenceID, idempotencyKey string) error {
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
		if idempotencyKey != "" {
			transaction.IdempotencyKey = &idempotencyKey
		}

		return tx.Create(transaction).Error
	})
	if err != nil {
		if txCtx.Err() != nil {
			recordLockTimeout("add_balance")
		}
		// Duplicate idempotency key — webhook redelivery. Swallow so the
		// upstream gets a 2xx and stops retrying.
		if idempotencyKey != "" && isDuplicateIdempotencyKey(err) {
			s.logger.Info("AddBalanceIdempotent: duplicate key, skipping",
				zap.String("user_id", userID.String()),
				zap.String("idempotency_key", idempotencyKey))
			return nil
		}
	}
	return err
}

// isDuplicateIdempotencyKey returns true if err comes from the partial unique
// index on transactions(idempotency_key). We string-match because the GORM
// driver wraps pgx errors; the Postgres SQLSTATE for unique_violation (23505)
// or the literal "duplicate key" text is stable across versions.
func isDuplicateIdempotencyKey(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "idempotency_key") {
		return false
	}
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "23505")
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
