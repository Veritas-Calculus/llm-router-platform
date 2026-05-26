package redeem

import (
	"testing"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRedeemTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			name TEXT,
			role TEXT,
			is_active BOOLEAN,
			require_password_change BOOLEAN,
			o_auth_provider TEXT,
			o_auth_id TEXT,
			last_login_at DATETIME,
			monthly_token_limit INTEGER DEFAULT 0,
			monthly_budget_usd REAL DEFAULT 0,
			rate_limit_per_minute INTEGER DEFAULT 0,
			balance REAL DEFAULT 0,
			tokens_invalidated_at DATETIME,
			email_verified BOOLEAN,
			email_verified_at DATETIME,
			mfa_enabled BOOLEAN,
			mfa_secret TEXT,
			mfa_backup_codes TEXT
		);
		CREATE TABLE redeem_codes (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			code TEXT NOT NULL,
			type TEXT NOT NULL,
			credit_amount REAL DEFAULT 0,
			plan_id TEXT,
			plan_days INTEGER DEFAULT 30,
			used_by_id TEXT,
			used_at DATETIME,
			expires_at DATETIME,
			is_active BOOLEAN DEFAULT true,
			batch_id TEXT,
			note TEXT
		);
		CREATE UNIQUE INDEX idx_redeem_codes_code ON redeem_codes(code);
		CREATE TABLE transactions (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			org_id TEXT NOT NULL,
			user_id TEXT,
			type TEXT NOT NULL,
			amount REAL NOT NULL,
			currency TEXT DEFAULT 'USD',
			balance REAL,
			description TEXT,
			reference_id TEXT,
			idempotency_key TEXT
		);
	`).Error)
	return db
}

func TestRedeemCreditAddsBalanceAndTransaction(t *testing.T) {
	db := setupRedeemTestDB(t)
	svc := NewService(db, zap.NewNop())
	userID := uuid.New()
	codeID := uuid.New()

	require.NoError(t, db.Create(&models.User{
		BaseModel:    models.BaseModel{ID: userID},
		Email:        "credit@example.com",
		PasswordHash: "hash",
		IsActive:     true,
	}).Error)
	require.NoError(t, db.Create(&models.RedeemCode{
		BaseModel:    models.BaseModel{ID: codeID},
		Code:         "ABCD-1234-EFGH",
		Type:         "credit",
		CreditAmount: models.MoneyFromFloat(12.3456789),
		IsActive:     true,
	}).Error)

	result, err := svc.Redeem(userID, " abcd-1234-efgh ")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.True(t, result.CreditAmount.Equal(models.MoneyFromFloat(12.3456789)))

	var user models.User
	require.NoError(t, db.First(&user, "id = ?", userID).Error)
	require.Equal(t, 12.3456789, models.MoneyToFloat(user.Balance))

	var tx models.Transaction
	require.NoError(t, db.First(&tx, "reference_id = ?", codeID.String()).Error)
	require.Equal(t, "recharge", tx.Type)
	require.Equal(t, 12.3456789, models.MoneyToFloat(tx.Amount))
	require.True(t, user.Balance.Equal(tx.Balance))
}

func TestRedeemLegacyPlanCodeWithoutPlanCreditsBalance(t *testing.T) {
	db := setupRedeemTestDB(t)
	svc := NewService(db, zap.NewNop())
	userID := uuid.New()
	codeID := uuid.New()

	require.NoError(t, db.Create(&models.User{
		BaseModel:    models.BaseModel{ID: userID},
		Email:        "legacy@example.com",
		PasswordHash: "hash",
		IsActive:     true,
	}).Error)
	require.NoError(t, db.Create(&models.RedeemCode{
		BaseModel:    models.BaseModel{ID: codeID},
		Code:         "PLAN-NOPE-CASH",
		Type:         "plan",
		CreditAmount: models.MoneyFromFloat(10),
		IsActive:     true,
	}).Error)

	result, err := svc.Redeem(userID, "PLAN-NOPE-CASH")
	require.NoError(t, err)
	require.True(t, result.Success)
	require.True(t, result.CreditAmount.Equal(models.MoneyFromFloat(10)))

	var user models.User
	require.NoError(t, db.First(&user, "id = ?", userID).Error)
	require.Equal(t, 10.0, models.MoneyToFloat(user.Balance))

	var redeemed models.RedeemCode
	require.NoError(t, db.First(&redeemed, "id = ?", codeID).Error)
	require.Equal(t, "credit", redeemed.Type)
	require.NotNil(t, redeemed.UsedByID)
}

func TestRedeemUnsupportedTypeDoesNotUseCode(t *testing.T) {
	db := setupRedeemTestDB(t)
	svc := NewService(db, zap.NewNop())
	userID := uuid.New()
	codeID := uuid.New()

	require.NoError(t, db.Create(&models.User{
		BaseModel:    models.BaseModel{ID: userID},
		Email:        "unsupported@example.com",
		PasswordHash: "hash",
		IsActive:     true,
	}).Error)
	require.NoError(t, db.Create(&models.RedeemCode{
		BaseModel:    models.BaseModel{ID: codeID},
		Code:         "BAD-TYPE-CODE",
		Type:         "coupon",
		CreditAmount: models.MoneyFromFloat(10),
		IsActive:     true,
	}).Error)

	result, err := svc.Redeem(userID, "BAD-TYPE-CODE")
	require.NoError(t, err)
	require.False(t, result.Success)
	require.Equal(t, "Unsupported code type", result.Message)

	var redeemed models.RedeemCode
	require.NoError(t, db.First(&redeemed, "id = ?", codeID).Error)
	require.Nil(t, redeemed.UsedByID)
}

func TestGenerateCodesRejectsInvalidCreditAndPlanInputs(t *testing.T) {
	db := setupRedeemTestDB(t)
	svc := NewService(db, zap.NewNop())

	_, err := svc.GenerateCodes("credit", models.MoneyFromFloat(0), nil, 30, 1, nil, "")
	require.ErrorContains(t, err, "credit amount must be positive")

	_, err = svc.GenerateCodes("plan", models.MoneyFromFloat(10), nil, 30, 1, nil, "")
	require.ErrorContains(t, err, "plan code requires a plan")
}
