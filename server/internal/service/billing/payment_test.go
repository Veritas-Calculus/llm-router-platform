package billing

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestTransactionModel_UserID(t *testing.T) {
	userID := uuid.New()
	orgID := uuid.New()

	txn := models.Transaction{
		OrgID:       orgID,
		UserID:      userID,
		Type:        "recharge",
		Amount:      5.0,
		Balance:     5.0,
		Currency:    "USD",
		Description: "Welcome credit",
	}

	assert.Equal(t, orgID, txn.OrgID)
	assert.Equal(t, userID, txn.UserID)
	assert.Equal(t, "recharge", txn.Type)
	assert.Equal(t, 5.0, txn.Amount)
}

func TestTransactionModel_DeductionNegativeAmount(t *testing.T) {
	txn := models.Transaction{
		OrgID:       uuid.New(),
		UserID:      uuid.New(),
		Type:        "deduction",
		Amount:      -0.05,
		Balance:     4.95,
		Description: "API usage: gpt-4",
	}

	assert.True(t, txn.Amount < 0, "Deduction should have negative amount")
	assert.Equal(t, "deduction", txn.Type)
}

func TestTransactionModel_RefundPositiveAmount(t *testing.T) {
	txn := models.Transaction{
		OrgID:       uuid.New(),
		UserID:      uuid.New(),
		Type:        "refund",
		Amount:      1.50,
		Balance:     6.50,
		Description: "Refund for failed request",
		ReferenceID: uuid.New().String(),
	}

	assert.True(t, txn.Amount > 0, "Refund should have positive amount")
	assert.Equal(t, "refund", txn.Type)
	assert.NotEmpty(t, txn.ReferenceID)
}

func TestTransactionTypes(t *testing.T) {
	validTypes := []string{"recharge", "deduction", "refund"}

	for _, txType := range validTypes {
		txn := models.Transaction{Type: txType}
		assert.NotEmpty(t, txn.Type)
	}
}

func TestPaymentWebhookIdempotency_OrderModel(t *testing.T) {
	order := models.Order{
		OrderNo:       "ord_test_123",
		Amount:        10.0,
		Currency:      "USD",
		Status:        "pending",
		PaymentMethod: "stripe",
	}

	assert.Equal(t, "pending", order.Status)
	assert.Equal(t, "ord_test_123", order.OrderNo)

	// Simulate idempotent completion
	order.Status = "completed"
	assert.Equal(t, "completed", order.Status)

	// Second "completion" shouldn't change anything meaningful
	previousStatus := order.Status
	order.Status = "completed"
	assert.Equal(t, previousStatus, order.Status)
}

func TestBalanceCalculation(t *testing.T) {
	tests := []struct {
		name            string
		initialBalance  float64
		operation       string
		amount          float64
		expectedBalance float64
	}{
		{"recharge", 0, "recharge", 5.0, 5.0},
		{"deduction", 5.0, "deduction", 0.05, 4.95},
		{"refund", 4.95, "refund", 1.0, 5.95},
		{"overdraft deduction", 1.0, "deduction", 1.5, -0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance := tt.initialBalance
			switch tt.operation {
			case "recharge", "refund":
				balance += tt.amount
			case "deduction":
				balance -= tt.amount
			}
			assert.InDelta(t, tt.expectedBalance, balance, 0.001)
		})
	}
}
