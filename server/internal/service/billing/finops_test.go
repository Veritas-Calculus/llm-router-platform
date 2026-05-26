package billing

import (
	"testing"

	"llm-router-platform/internal/models"
)

func TestMeanStdDev(t *testing.T) {
	tests := []struct {
		name       string
		values     []int64
		wantMean   float64
		wantStdDev float64
	}{
		{
			name:       "empty slice",
			values:     []int64{},
			wantMean:   0,
			wantStdDev: 0,
		},
		{
			name:       "single value",
			values:     []int64{5},
			wantMean:   5.0,
			wantStdDev: 0,
		},
		{
			name:       "uniform values",
			values:     []int64{10, 10, 10, 10},
			wantMean:   10.0,
			wantStdDev: 0,
		},
		{
			name:       "known distribution",
			values:     []int64{2, 4, 4, 4, 5, 5, 7, 9},
			wantMean:   5.0,
			wantStdDev: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMean, gotStdDev := meanStdDevUnits(tt.values)
			if gotMean != tt.wantMean {
				t.Errorf("meanStdDevUnits() mean = %v, want %v", gotMean, tt.wantMean)
			}
			// Allow small floating point difference for stddev
			diff := gotStdDev - tt.wantStdDev
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("meanStdDevUnits() stddev = %v, want %v", gotStdDev, tt.wantStdDev)
			}
		})
	}
}

func TestBudgetStatusFields(t *testing.T) {
	status := &BudgetStatus{
		CurrentSpend:   models.MoneyFromFloat(50.0),
		RemainingUSD:   models.MoneyFromFloat(50.0),
		UsagePercent:   50.0,
		IsOverBudget:   false,
		IsAlertTripped: false,
		PeriodStart:    "2026-03-01",
		PeriodEnd:      "2026-03-31",
	}

	if status.IsOverBudget {
		t.Error("should not be over budget at 50%")
	}
	if status.IsAlertTripped {
		t.Error("should not be alert tripped at 50%")
	}
	if !status.RemainingUSD.Equal(models.MoneyFromFloat(50.0)) {
		t.Errorf("remaining = %v, want 50", status.RemainingUSD)
	}
}

func TestAnomalyResultFields(t *testing.T) {
	result := &AnomalyResult{
		IsAnomaly:    true,
		CurrentCost:  models.MoneyFromFloat(100.0),
		ExpectedCost: models.MoneyFromFloat(10.0),
		Deviation:    5.0,
		Threshold:    3.0,
		WindowDays:   14,
		Message:      "cost anomaly detected",
	}

	if !result.IsAnomaly {
		t.Error("should be anomaly")
	}
	if result.Deviation <= result.Threshold {
		t.Error("deviation should exceed threshold for anomaly")
	}
}
