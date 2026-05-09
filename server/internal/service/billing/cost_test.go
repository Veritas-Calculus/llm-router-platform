package billing

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestRoundCost(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{0, 0},
		{0.0000001, 0},                     // truncates below 6 decimals
		{0.0000005, 0.000001},               // rounds up at the boundary
		{0.123456789, 0.123457},
		{1.0 / 3.0, 0.333333},
		{-0.0000005, -0.000001},
	}
	for _, c := range cases {
		got := roundCost(c.in)
		// 1e-9 absolute tolerance — anything looser would defeat the purpose.
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("roundCost(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestRoundCostStable(t *testing.T) {
	// Sum many small charges and verify the running sum stays bit-exact when
	// rounded after each step.
	charge := roundCost(0.000001)
	var total float64
	for range 1_000_000 {
		total = roundCost(total + charge)
	}
	assert.InDelta(t, 1.0, total, 1e-6)
}

func TestCalculateCustomerChargeMixedDimensions(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:  0.001,
		OutputPricePer1K: 0.002,
		PricePerSecond:   0.0001,
		PricePerImage:    0.04,
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
	assert.InDelta(t, want, got, 1e-9)
}

func TestCalculateProviderCostFallsBackWhenUnconfigured(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:  0.01,
		OutputPricePer1K: 0.02,
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
	assert.Equal(t, customer, provider)
}

func TestCalculateProviderCostUsesProviderRatesWhenSet(t *testing.T) {
	model := &models.Model{
		InputPricePer1K:        0.01,
		OutputPricePer1K:       0.02,
		ProviderInputCostPer1K: 0.005,
		ProviderOutputCostPer1K: 0.01,
	}
	log := &models.UsageLog{
		RequestTokens:  1_000,
		ResponseTokens: 1_000,
	}
	s := &Service{}
	customer := s.calculateCustomerCharge(model, log)
	provider := s.calculateProviderCost(model, log, customer)
	// Provider half-priced relative to customer.
	assert.InDelta(t, customer/2, provider, 1e-9)
}
