package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"llm-router-platform/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPaymentTransactionResponseKeepsNumericMoneyJSON(t *testing.T) {
	txs := []models.Transaction{
		{
			OrgID:    uuid.New(),
			UserID:   uuid.New(),
			Type:     "recharge",
			Amount:   models.MoneyFromFloat(12.3456789),
			Balance:  models.MoneyFromFloat(42.5),
			Currency: "USD",
		},
	}

	payload, err := json.Marshal(toPaymentTransactionResponses(txs))
	require.NoError(t, err)

	body := string(payload)
	require.Contains(t, body, `"amount":12.3456789`)
	require.Contains(t, body, `"balance":42.5`)
	require.False(t, strings.Contains(body, `"amount":"`))
	require.False(t, strings.Contains(body, `"balance":"`))
}

func TestPaymentOrderResponseKeepsNumericMoneyJSON(t *testing.T) {
	orders := []models.Order{
		{
			OrgID:         uuid.New(),
			OrderNo:       "RECH-1",
			Amount:        models.MoneyFromFloat(19.99),
			Currency:      "USD",
			Status:        "pending",
			PaymentMethod: "stripe",
		},
	}

	payload, err := json.Marshal(toPaymentOrderResponses(orders))
	require.NoError(t, err)

	body := string(payload)
	require.Contains(t, body, `"amount":19.99`)
	require.False(t, strings.Contains(body, `"amount":"`))
}

func TestPaymentMoneyInputAcceptsStringAndNumber(t *testing.T) {
	var stringReq struct {
		Amount paymentMoney `json:"amount"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"amount":"12.3400"}`), &stringReq))
	require.True(t, stringReq.Amount.money().Equal(models.MoneyFromFloat(12.34)))

	var numberReq struct {
		Amount paymentMoney `json:"amount"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"amount":0.123456789}`), &numberReq))
	require.True(t, numberReq.Amount.money().Equal(models.MoneyFromFloat(0.123456789)))
}
