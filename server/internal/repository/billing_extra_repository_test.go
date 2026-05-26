package repository

import (
	"context"
	"testing"
	"time"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestFulfillRechargeOrderIsAtomicAndIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, createBillingFulfillmentTables(db))

	userID := uuid.New()
	orderID := uuid.New()
	orderNo := "ord_" + uuid.NewString()
	now := time.Now()
	require.NoError(t, db.Exec(`INSERT INTO users (id, balance) VALUES (?, ?)`, userID.String(), 10.0).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO orders (
			id, created_at, updated_at, org_id, order_no, amount, currency,
			status, payment_method, external_id
		) VALUES (?, ?, ?, ?, ?, ?, 'USD', 'pending', 'stripe', 'cs_test')
	`, orderID.String(), now, now, userID.String(), orderNo, 25.0).Error)

	repo := NewSubscriptionRepository(db)
	order, err := repo.GetOrderByNo(context.Background(), orderNo)
	require.NoError(t, err)

	require.NoError(t, repo.FulfillRechargeOrder(
		context.Background(),
		userID,
		models.MoneyFromFloat(25.0),
		"recharge",
		"Credit Top-up via Stripe",
		order,
		"stripe:"+orderNo,
	))

	var balance float64
	require.NoError(t, db.Raw(`SELECT balance FROM users WHERE id = ?`, userID.String()).Scan(&balance).Error)
	require.InDelta(t, 35.0, balance, 0.0001)

	var status string
	require.NoError(t, db.Raw(`SELECT status FROM orders WHERE order_no = ?`, orderNo).Scan(&status).Error)
	require.Equal(t, "paid", status)

	var txCount int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM transactions WHERE idempotency_key = ?`, "stripe:"+orderNo).Scan(&txCount).Error)
	require.Equal(t, int64(1), txCount)

	// A redelivered provider webhook with the same idempotency key must not
	// credit the balance again.
	order.Status = "pending"
	require.NoError(t, repo.FulfillRechargeOrder(
		context.Background(),
		userID,
		models.MoneyFromFloat(25.0),
		"recharge",
		"Credit Top-up via Stripe",
		order,
		"stripe:"+orderNo,
	))

	require.NoError(t, db.Raw(`SELECT balance FROM users WHERE id = ?`, userID.String()).Scan(&balance).Error)
	require.InDelta(t, 35.0, balance, 0.0001)
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM transactions WHERE idempotency_key = ?`, "stripe:"+orderNo).Scan(&txCount).Error)
	require.Equal(t, int64(1), txCount)
}

func createBillingFulfillmentTables(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			balance REAL NOT NULL DEFAULT 0
		)
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE TABLE orders (
			id TEXT PRIMARY KEY,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			org_id TEXT NOT NULL,
			plan_id TEXT,
			order_no TEXT NOT NULL UNIQUE,
			amount REAL NOT NULL,
			currency TEXT,
			status TEXT,
			payment_method TEXT,
			external_id TEXT
		)
	`).Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE TABLE transactions (
			id TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(16)))),
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME,
			org_id TEXT NOT NULL,
			user_id TEXT,
			type TEXT NOT NULL,
			amount REAL NOT NULL,
			currency TEXT,
			balance REAL,
			description TEXT,
			reference_id TEXT,
			idempotency_key TEXT
		)
	`).Error; err != nil {
		return err
	}
	return db.Exec(`
		CREATE UNIQUE INDEX idx_transactions_idempotency_key
		ON transactions(idempotency_key)
		WHERE idempotency_key IS NOT NULL
	`).Error
}
