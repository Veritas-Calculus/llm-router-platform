package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
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

// DeductBalanceMoney subtracts the cost of a request using decimal money.
//
// The deduction and the matching transaction record are committed atomically
// under a row lock on the user. The low-balance alert is dispatched only
// AFTER the transaction commits — sending it inside the closure would still
// fire on rollback, producing false alerts. The lock acquisition is bounded
// by lockTimeout so a stuck peer cannot pin a request goroutine indefinitely.
func (s *BalanceService) DeductBalanceMoney(ctx context.Context, userID uuid.UUID, amount decimal.Decimal, description string, referenceID string) error {
	amount = amount.Round(models.MoneyScale)
	if !amount.IsPositive() {
		return nil
	}

	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	var (
		alertEmail   string
		alertName    string
		alertBalance decimal.Decimal
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
		newBalance := models.MoneySub(user.Balance, amount)
		if newBalance.IsNegative() {
			return ErrInsufficientBalance
		}
		user.Balance = newBalance
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance", user.Balance).Error; err != nil {
			return err
		}

		transaction := &models.Transaction{
			OrgID:       userID,
			UserID:      userID,
			Type:        "deduction",
			Amount:      models.MoneyNeg(amount),
			Balance:     user.Balance,
			Description: description,
			ReferenceID: referenceID,
		}

		if err := tx.Create(transaction).Error; err != nil {
			return err
		}

		if user.Balance.Cmp(lowBalanceThreshold) < 0 {
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

// AddBalanceMoney adds decimal credits to the user's balance.
func (s *BalanceService) AddBalanceMoney(ctx context.Context, userID uuid.UUID, amount decimal.Decimal, txType string, description string, referenceID string) error {
	return s.addBalance(ctx, userID, amount, txType, description, referenceID, "")
}

// AddBalanceIdempotent adds credits and attaches an idempotency key. A second
// call with the same key — typically the upstream provider's event/transaction
// id, e.g. Stripe evt_…, WeChat transaction_id, Alipay trade_no — is rejected
// by the partial unique index on transactions(idempotency_key) and surfaces as
// a no-op (returns nil so the webhook can ack the duplicate retry).
func (s *BalanceService) AddBalanceMoneyIdempotent(ctx context.Context, userID uuid.UUID, amount decimal.Decimal, txType, description, referenceID, idempotencyKey string) error {
	return s.addBalance(ctx, userID, amount, txType, description, referenceID, idempotencyKey)
}

func (s *BalanceService) addBalance(ctx context.Context, userID uuid.UUID, amount decimal.Decimal, txType, description, referenceID, idempotencyKey string) error {
	amount = amount.Round(models.MoneyScale)
	if !amount.IsPositive() {
		return fmt.Errorf("amount must be positive")
	}

	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	err := s.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			return err
		}

		user.Balance = models.MoneyAdd(user.Balance, amount)
		if err := tx.Model(&models.User{}).Where("id = ?", userID).Update("balance", user.Balance).Error; err != nil {
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
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "idempotency_key") {
		return false
	}
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "23505") ||
		strings.Contains(msg, "unique constraint failed")
}

// WelcomeCreditAmount is the amount granted on email verification (C-01).
// Kept here so the resolver, frontend, and tests all reference the same
// constant. If you change this, also update the marketing copy on the
// frontend (LoginPage / VerifyEmailPage) so the user-visible promise of
// "$5 welcome credit" stays accurate.
var WelcomeCreditAmount = decimal.NewFromInt(5).Round(models.MoneyScale)

// GrantWelcomeCredit credits the post-verification $5 welcome bonus exactly
// once per user. The DB transaction takes a row lock on the user, checks
// welcome_credit_granted_at IS NULL, sets it to now(), bumps the balance,
// and writes the matching transactions row — all atomically so a concurrent
// re-verify cannot double-credit.
func (s *BalanceService) GrantWelcomeCredit(ctx context.Context, u *models.User) error {
	if u == nil {
		return fmt.Errorf("nil user")
	}
	amount := WelcomeCreditAmount

	txCtx, cancel := context.WithTimeout(ctx, lockTimeout)
	defer cancel()

	return s.db.WithContext(txCtx).Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", u.ID).Error; err != nil {
			return fmt.Errorf("load user: %w", err)
		}
		if user.WelcomeCreditGrantedAt != nil {
			// Race-lost: another call beat us to it. Treat as success
			// so the resolver returns a clean no-op.
			return nil
		}

		newBalance := models.MoneyAdd(user.Balance, amount)
		now := s.now()
		updates := map[string]interface{}{
			"balance":                   newBalance,
			"welcome_credit_granted_at": now,
		}
		if err := tx.Model(&models.User{}).Where("id = ?", u.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("update user: %w", err)
		}

		transaction := &models.Transaction{
			OrgID:       u.ID,
			UserID:      u.ID,
			Type:        "recharge",
			Amount:      amount,
			Balance:     newBalance,
			Description: "Welcome credit",
			Currency:    "USD",
		}
		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("write transaction: %w", err)
		}

		s.logger.Info("welcome credit granted",
			zap.String("user_id", u.ID.String()),
			zap.String("amount", amount.StringFixed(models.MoneyScale)),
		)
		return nil
	})
}

// now is split out so tests can substitute a fixed clock if needed.
func (s *BalanceService) now() time.Time { return time.Now() }

// GetBalanceMoney returns the user's balance without converting it through float64.
func (s *BalanceService) GetBalanceMoney(ctx context.Context, userID uuid.UUID) (decimal.Decimal, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return decimal.Zero, err
	}
	return user.Balance.Round(models.MoneyScale), nil
}

func (s *BalanceService) GetTransactions(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Transaction, int64, error) {
	return s.txRepo.GetByUserID(ctx, userID, limit, offset)
}
