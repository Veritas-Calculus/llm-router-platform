package billing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateAlipayNotifyAmountAccepts(t *testing.T) {
	got, err := validateAlipayNotifyAmount("12.34", 12.34)
	require.NoError(t, err)
	assert.InDelta(t, 12.34, got, 1e-9)
}

func TestValidateAlipayNotifyAmountToleratesPenny(t *testing.T) {
	// 1¢ tolerance for float-string round-trip.
	got, err := validateAlipayNotifyAmount("12.345", 12.34)
	require.NoError(t, err)
	assert.InDelta(t, 12.345, got, 1e-9)
}

func TestValidateAlipayNotifyAmountRejectsMismatch(t *testing.T) {
	_, err := validateAlipayNotifyAmount("100.00", 10.00)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount mismatch")
}

func TestValidateAlipayNotifyAmountRejectsBlank(t *testing.T) {
	_, err := validateAlipayNotifyAmount("", 10.00)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid total_amount")
}

func TestValidateAlipayNotifyAmountRejectsGarbage(t *testing.T) {
	_, err := validateAlipayNotifyAmount("not-a-number", 10.00)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid total_amount")
}

func TestValidateAlipayNotifyAmountRejectsNegative(t *testing.T) {
	_, err := validateAlipayNotifyAmount("-50.00", 50.00)
	require.Error(t, err, "negative amounts must not match a positive order amount")
}
