package billing

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestMoneyFromFloatRoundsToDBScale(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{0.000000001, 0},          // below NUMERIC(20,8) scale
		{0.000000005, 0.00000001}, // rounds up at the DB scale boundary
		{0.123456789, 0.12345679},
		{1.0 / 3.0, 0.33333333},
		{-0.000000005, -0.00000001},
	}
	for _, c := range cases {
		got := models.MoneyToFloat(models.MoneyFromFloat(c.in))
		assert.InDelta(t, c.want, got, 1e-9)
	}
}

func TestMoneyAddStable(t *testing.T) {
	// Sum many small charges and verify the running sum stays bit-exact when
	// rounded after each step.
	charge := models.MoneyFromFloat(0.00000001)
	total := models.MoneyFromFloat(0)
	for range 1_000_000 {
		total = models.MoneyAdd(total, charge)
	}
	assert.True(t, total.Equal(models.MoneyFromFloat(0.01)))
}

func TestMoneyArithmeticUsesDecimalBeforeRounding(t *testing.T) {
	total := models.MoneyAdd(models.MoneyFromFloat(0.1), models.MoneyFromFloat(0.2))
	assert.Equal(t, 0.3, models.MoneyToFloat(total))

	remaining := models.MoneySub(total, models.MoneyFromFloat(0.1))
	assert.Equal(t, 0.2, models.MoneyToFloat(remaining))

	assert.True(t, models.MoneyNeg(models.MoneyFromFloat(0.1)).Equal(models.MoneyFromFloat(-0.1)))
}

func TestCalculateCustomerChargeMixedDimensions(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:  models.MoneyFromFloat(0.001),
		OutputPricePer1K: models.MoneyFromFloat(0.002),
		PricePerSecond:   models.MoneyFromFloat(0.0001),
		PricePerImage:    models.MoneyFromFloat(0.04),
	}
	log := &models.UsageLog{
		RequestTokens:  1_000,
		ResponseTokens: 500,
		DurationMs:     10_000, // 10 seconds
		ItemCount:      2,
	}
	s := &Service{}
	got := s.calculateCustomerCharge(model, log)
	// 0.001 (input) + 0.001 (output) + 0.001 (10s * 0.0001) + 0.08 (2 * 0.04)
	want := 0.001 + 0.001 + 0.001 + 0.08
	assert.InDelta(t, want, models.MoneyToFloat(got), 1e-9)
}

func TestCalculateProviderCostFallsBackWhenUnconfigured(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:  models.MoneyFromFloat(0.01),
		OutputPricePer1K: models.MoneyFromFloat(0.02),
	}
	log := &models.UsageLog{
		RequestTokens:  1_000,
		ResponseTokens: 1_000,
	}
	s := &Service{}
	customer := s.calculateCustomerCharge(model, log)
	provider := s.calculateProviderCost(model, log, customer)
	// All provider rates are zero, so we fall back to the customer charge to
	// avoid reporting 100% margin on unconfigured models.
	assert.True(t, customer.Equal(provider))
}

func TestCalculateProviderCostUsesProviderRatesWhenSet(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:         models.MoneyFromFloat(0.01),
		OutputPricePer1K:        models.MoneyFromFloat(0.02),
		ProviderInputCostPer1K:  models.MoneyFromFloat(0.005),
		ProviderOutputCostPer1K: models.MoneyFromFloat(0.01),
	}
	log := &models.UsageLog{
		RequestTokens:  1_000,
		ResponseTokens: 1_000,
	}
	s := &Service{}
	customer := s.calculateCustomerCharge(model, log)
	provider := s.calculateProviderCost(model, log, customer)
	// Provider half-priced relative to customer.
	assert.InDelta(t, models.MoneyToFloat(customer)/2, models.MoneyToFloat(provider), 1e-9)
}
