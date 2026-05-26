package billing

import (
	"testing"

	"llm-router-platform/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAlipayNotifyAmountAccepts(t *testing.T) {
	got, err := validateAlipayNotifyAmount("12.34", models.MoneyFromFloat(12.34))
	require.NoError(t, err)
	assert.True(t, got.Equal(models.MoneyFromFloat(12.34)))
}

func TestValidateAlipayNotifyAmountAcceptsEquivalentScale(t *testing.T) {
	got, err := validateAlipayNotifyAmount("12.3400", models.MoneyFromFloat(12.34))
	require.NoError(t, err)
	assert.True(t, got.Equal(models.MoneyFromFloat(12.34)))
}

func TestValidateAlipayNotifyAmountRejectsFractionalCent(t *testing.T) {
	_, err := validateAlipayNotifyAmount("12.345", models.MoneyFromFloat(12.34))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "precision exceeds cents")
}

func TestValidateAlipayNotifyAmountRejectsMismatch(t *testing.T) {
	_, err := validateAlipayNotifyAmount("100.00", models.MoneyFromFloat(10.00))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount mismatch")
}

func TestValidateAlipayNotifyAmountRejectsBlank(t *testing.T) {
	_, err := validateAlipayNotifyAmount("", models.MoneyFromFloat(10.00))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid total_amount")
}

func TestValidateAlipayNotifyAmountRejectsGarbage(t *testing.T) {
	_, err := validateAlipayNotifyAmount("not-a-number", models.MoneyFromFloat(10.00))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid total_amount")
}

func TestValidateAlipayNotifyAmountRejectsNegative(t *testing.T) {
	_, err := validateAlipayNotifyAmount("-50.00", models.MoneyFromFloat(50.00))
	require.Error(t, err, "negative amounts must not match a positive order amount")
	assert.Contains(t, err.Error(), "must be positive")
}
