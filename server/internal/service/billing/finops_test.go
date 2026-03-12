package billing

import (
	"testing"
)

func TestMeanStdDev(t *testing.T) {
	tests := []struct {
		name       string
		values     []float64
		wantMean   float64
		wantStdDev float64
	}{
		{
			name:       "empty slice",
			values:     []float64{},
			wantMean:   0,
			wantStdDev: 0,
		},
		{
			name:       "single value",
			values:     []float64{5.0},
			wantMean:   5.0,
			wantStdDev: 0,
		},
		{
			name:       "uniform values",
			values:     []float64{10.0, 10.0, 10.0, 10.0},
			wantMean:   10.0,
			wantStdDev: 0,
		},
		{
			name:       "known distribution",
			values:     []float64{2, 4, 4, 4, 5, 5, 7, 9},
			wantMean:   5.0,
			wantStdDev: 2.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMean, gotStdDev := meanStdDev(tt.values)
			if gotMean != tt.wantMean {
				t.Errorf("meanStdDev() mean = %v, want %v", gotMean, tt.wantMean)
			}
			// Allow small floating point difference for stddev
			diff := gotStdDev - tt.wantStdDev
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("meanStdDev() stddev = %v, want %v", gotStdDev, tt.wantStdDev)
			}
		})
	}
}

func TestBudgetStatusFields(t *testing.T) {
	status := &BudgetStatus{
		CurrentSpend:   50.0,
		RemainingUSD:   50.0,
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
	if status.RemainingUSD != 50.0 {
		t.Errorf("remaining = %v, want 50", status.RemainingUSD)
	}
}

func TestAnomalyResultFields(t *testing.T) {
	result := &AnomalyResult{
		IsAnomaly:    true,
		CurrentCost:  100.0,
		ExpectedCost: 10.0,
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
