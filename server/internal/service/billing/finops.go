// Package billing provides billing, usage tracking, and FinOps features.
// This file implements budget management and cost anomaly detection.
package billing

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"strconv"
	"time"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ─── Budget Management ──────────────────────────────────────

// BudgetStatus represents the current spend vs. budget.
type BudgetStatus struct {
	Budget         *models.Budget  `json:"budget"`
	CurrentSpend   decimal.Decimal `json:"current_spend"`
	RemainingUSD   decimal.Decimal `json:"remaining_usd"`
	UsagePercent   float64         `json:"usage_percent"`
	IsOverBudget   bool            `json:"is_over_budget"`
	IsAlertTripped bool            `json:"is_alert_tripped"`
	PeriodStart    string          `json:"period_start"`
	PeriodEnd      string          `json:"period_end"`
}

// BudgetService handles budget creation, checking, and alerting.
type BudgetService struct {
	usageRepo  *repository.UsageLogRepository
	budgetRepo *repository.BudgetRepository
	logger     *zap.Logger
}

// NewBudgetService creates a new budget service.
func NewBudgetService(usageRepo *repository.UsageLogRepository, budgetRepo *repository.BudgetRepository, logger *zap.Logger) *BudgetService {
	return &BudgetService{
		usageRepo:  usageRepo,
		budgetRepo: budgetRepo,
		logger:     logger,
	}
}

// SetBudget creates or updates a budget for a user.
//
// `threshold` is the alert threshold expressed as a fraction in [0, 1]
// (matching the `Budget.AlertThreshold` storage convention). `enforceHardLimit`
// engages the request-time middleware that returns 429 once `currentSpend`
// reaches `limitUSD` — without it the budget is alert-only.
func (s *BudgetService) SetBudget(ctx context.Context, userID uuid.UUID, limitUSD decimal.Decimal, threshold float64, enforceHardLimit bool, webhookURL, email string) (*models.Budget, error) {
	limitUSD = limitUSD.Round(models.MoneyScale)
	budget := &models.Budget{
		OrgID:            userID,
		MonthlyLimitUSD:  limitUSD,
		AlertThreshold:   threshold,
		EnforceHardLimit: enforceHardLimit,
		IsActive:         true,
		WebhookURL:       webhookURL,
		Email:            email,
	}
	if err := s.budgetRepo.Upsert(ctx, budget); err != nil {
		s.logger.Error("failed to save budget", zap.Error(err))
		return nil, fmt.Errorf("failed to save budget: %w", err)
	}
	s.logger.Info("budget set",
		zap.String("user_id", userID.String()),
		zap.String("limit_usd", limitUSD.StringFixed(2)),
		zap.Float64("threshold", threshold),
		zap.Bool("enforce_hard_limit", enforceHardLimit),
	)
	// Re-read from DB to get the generated ID/timestamps
	saved, err := s.budgetRepo.GetByUserID(ctx, userID)
	if err != nil {
		return budget, nil
	}
	return saved, nil
}

// GetBudget returns the budget for a user.
func (s *BudgetService) GetBudget(ctx context.Context, userID uuid.UUID) *models.Budget {
	budget, err := s.budgetRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil
	}
	return budget
}

// DeleteBudget removes a budget for a user.
func (s *BudgetService) DeleteBudget(ctx context.Context, userID uuid.UUID) error {
	return s.budgetRepo.DeleteByUserID(ctx, userID)
}

// CheckBudget evaluates current spend vs budget and returns status.
func (s *BudgetService) CheckBudget(ctx context.Context, userID uuid.UUID) (*BudgetStatus, error) {
	budget, err := s.budgetRepo.GetByUserID(ctx, userID)
	if err != nil || budget == nil || !budget.IsActive {
		return nil, nil
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	// Use SQL SUM aggregation instead of loading all rows
	row, err := s.usageRepo.AggregateByTimeRange(ctx, &userID, nil, nil, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate usage: %w", err)
	}

	currentSpendMoney := row.TotalCost.Round(models.MoneyScale)
	remaining := budget.MonthlyLimitUSD.Sub(currentSpendMoney).Round(models.MoneyScale)
	if remaining.IsNegative() {
		remaining = decimal.Zero
	}

	usageRatio := decimal.Zero
	if budget.MonthlyLimitUSD.IsPositive() {
		usageRatio = currentSpendMoney.Div(budget.MonthlyLimitUSD)
	}
	usagePercent, _ := usageRatio.Mul(decimal.NewFromInt(100)).Round(2).Float64()

	status := &BudgetStatus{
		Budget:         budget,
		CurrentSpend:   currentSpendMoney,
		RemainingUSD:   remaining,
		UsagePercent:   usagePercent,
		IsOverBudget:   budget.MonthlyLimitUSD.IsPositive() && currentSpendMoney.Cmp(budget.MonthlyLimitUSD) >= 0,
		IsAlertTripped: usageRatio.Cmp(decimal.NewFromFloat(budget.AlertThreshold)) >= 0,
		PeriodStart:    periodStart.Format("2006-01-02"),
		PeriodEnd:      periodEnd.Format("2006-01-02"),
	}

	if status.IsAlertTripped {
		s.logger.Warn("budget alert triggered",
			zap.String("user_id", userID.String()),
			zap.String("spend", currentSpendMoney.StringFixed(2)),
			zap.String("limit", budget.MonthlyLimitUSD.StringFixed(2)),
			zap.Float64("percent", status.UsagePercent),
		)
	}

	return status, nil
}

// ─── Cost Anomaly Detection ─────────────────────────────────

// AnomalyResult represents the output of anomaly detection.
type AnomalyResult struct {
	IsAnomaly    bool            `json:"is_anomaly"`
	CurrentCost  decimal.Decimal `json:"current_cost"`
	ExpectedCost decimal.Decimal `json:"expected_cost"` // Mean of historical
	Deviation    float64         `json:"deviation"`     // Standard deviations from mean
	Threshold    float64         `json:"threshold"`     // σ threshold used
	WindowDays   int             `json:"window_days"`
	Message      string          `json:"message,omitempty"`
}

// DetectCostAnomaly compares today's cost against a sliding window.
// Returns anomaly if current day cost exceeds mean + threshold*σ.
// Uses SQL daily aggregation to avoid loading individual rows.
func (s *Service) DetectCostAnomaly(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, windowDays int, sigmaThreshold float64) (*AnomalyResult, error) {
	if windowDays <= 1 {
		windowDays = 14 // default 14-day window
	}
	if sigmaThreshold <= 0 {
		sigmaThreshold = 3.0 // default 3σ
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowStart := todayStart.AddDate(0, 0, -windowDays)

	// Use SQL aggregation: one row per day instead of loading all individual logs
	dailyRows, err := s.usageRepo.AggregateDailyByTimeRange(ctx, &orgID, projectID, nil, windowStart, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate daily usage: %w", err)
	}

	// Build daily cost map from SQL result as fixed-scale integer units.
	dailyCostUnits := make(map[string]int64, len(dailyRows))
	for _, row := range dailyRows {
		dailyCostUnits[row.Date] = models.MoneyToUnits(row.Cost)
	}

	todayKey := todayStart.Format("2006-01-02")
	todayCostUnits := dailyCostUnits[todayKey]
	todayCostMoney := models.MoneyFromUnits(todayCostUnits)

	// Build historical costs (excluding today), filling zero-cost days
	var historicalCostUnits []int64
	for d := 1; d <= windowDays; d++ {
		dayKey := todayStart.AddDate(0, 0, -d).Format("2006-01-02")
		historicalCostUnits = append(historicalCostUnits, dailyCostUnits[dayKey]) // 0 if missing
	}

	if len(historicalCostUnits) < 3 {
		return &AnomalyResult{
			IsAnomaly:    false,
			CurrentCost:  todayCostMoney.Round(models.MoneyScale),
			ExpectedCost: decimal.Zero,
			WindowDays:   windowDays,
			Message:      "insufficient data for anomaly detection",
		}, nil
	}

	meanUnits, stddevUnits := meanStdDevUnits(historicalCostUnits)

	deviation := 0.0
	if stddevUnits > 0 {
		deviation = (float64(todayCostUnits) - meanUnits) / stddevUnits
	}

	isAnomaly := deviation > sigmaThreshold
	expectedCost := statUnitsMoney(meanUnits)

	result := &AnomalyResult{
		IsAnomaly:    isAnomaly,
		CurrentCost:  todayCostMoney.Round(4).Round(models.MoneyScale),
		ExpectedCost: expectedCost,
		Deviation:    math.Round(deviation*100) / 100,
		Threshold:    sigmaThreshold,
		WindowDays:   windowDays,
	}

	if isAnomaly {
		result.Message = fmt.Sprintf("cost anomaly detected: $%s is %.1fσ above expected $%s", todayCostMoney.Round(4).StringFixed(4), deviation, expectedCost.StringFixed(4))
		s.logger.Warn("cost anomaly detected",
			zap.String("org_id", orgID.String()),
			zap.String("today_cost", todayCostMoney.Round(4).StringFixed(4)),
			zap.String("expected", expectedCost.StringFixed(4)),
			zap.Float64("sigma", deviation),
		)
	}

	return result, nil
}

// DetectSystemCostAnomaly runs anomaly detection across all system usage.
func (s *Service) DetectSystemCostAnomaly(ctx context.Context, windowDays int, sigmaThreshold float64) (*AnomalyResult, error) {
	if windowDays <= 1 {
		windowDays = 14
	}
	if sigmaThreshold <= 0 {
		sigmaThreshold = 3.0
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowStart := todayStart.AddDate(0, 0, -windowDays)

	logs, err := s.usageRepo.GetByTimeRange(ctx, windowStart, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage logs: %w", err)
	}

	dailyCostUnits := make(map[string]int64)
	var todayCostUnits int64
	todayKey := todayStart.Format("2006-01-02")

	for _, log := range logs {
		dayKey := log.CreatedAt.Format("2006-01-02")
		costUnits := models.MoneyToUnits(log.Cost)
		dailyCostUnits[dayKey] += costUnits
		if dayKey == todayKey {
			todayCostUnits += costUnits
		}
	}
	todayCostMoney := models.MoneyFromUnits(todayCostUnits)

	var historicalCostUnits []int64
	for key, costUnits := range dailyCostUnits {
		if key != todayKey {
			historicalCostUnits = append(historicalCostUnits, costUnits)
		}
	}

	for d := 0; d < windowDays; d++ {
		dayKey := todayStart.AddDate(0, 0, -d-1).Format("2006-01-02")
		if _, exists := dailyCostUnits[dayKey]; !exists {
			historicalCostUnits = append(historicalCostUnits, 0)
		}
	}

	if len(historicalCostUnits) < 3 {
		return &AnomalyResult{
			IsAnomaly:   false,
			CurrentCost: todayCostMoney.Round(models.MoneyScale),
			WindowDays:  windowDays,
			Message:     "insufficient data for anomaly detection",
		}, nil
	}

	meanUnits, stddevUnits := meanStdDevUnits(historicalCostUnits)
	deviation := 0.0
	if stddevUnits > 0 {
		deviation = (float64(todayCostUnits) - meanUnits) / stddevUnits
	}
	expectedCost := statUnitsMoney(meanUnits)

	return &AnomalyResult{
		IsAnomaly:    deviation > sigmaThreshold,
		CurrentCost:  todayCostMoney.Round(4).Round(models.MoneyScale),
		ExpectedCost: expectedCost,
		Deviation:    math.Round(deviation*100) / 100,
		Threshold:    sigmaThreshold,
		WindowDays:   windowDays,
		Message: func() string {
			if deviation > sigmaThreshold {
				return fmt.Sprintf("system cost anomaly: $%s is %.1fσ above expected $%s", todayCostMoney.Round(4).StringFixed(4), deviation, expectedCost.StringFixed(4))
			}
			return ""
		}(),
	}, nil
}

// ─── CSV Export ──────────────────────────────────────────────

const csvBatchSize = 1000 // rows per batch for streaming export

// ExportUsageCSV writes usage logs to a CSV writer in streaming batches.
func (s *Service) ExportUsageCSV(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time, w *csv.Writer) error {
	// Write header
	header := []string{
		"Timestamp", "Model", "Input Tokens", "Output Tokens", "Total Tokens",
		"Cost (USD)", "Latency (ms)", "Status Code", "Error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	// Stream in batches to avoid OOM
	offset := 0
	for {
		logs, err := s.usageRepo.GetByOrgOrProjectPaginated(ctx, &userID, nil, nil, startTime, endTime, csvBatchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to get usage logs (offset %d): %w", offset, err)
		}
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			row := []string{
				log.CreatedAt.Format(time.RFC3339),
				log.ModelName,
				strconv.Itoa(log.RequestTokens),
				strconv.Itoa(log.ResponseTokens),
				strconv.Itoa(log.TotalTokens),
				log.Cost.Round(6).StringFixed(6),
				strconv.FormatInt(log.Latency, 10),
				strconv.Itoa(log.StatusCode),
				log.ErrorMessage,
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		if len(logs) < csvBatchSize {
			break
		}
		offset += csvBatchSize
	}

	return nil
}

// ExportSystemUsageCSV writes system-wide usage to CSV in streaming batches.
func (s *Service) ExportSystemUsageCSV(ctx context.Context, startTime, endTime time.Time, w *csv.Writer) error {
	header := []string{
		"Timestamp", "User ID", "API Key ID", "Model", "Input Tokens", "Output Tokens",
		"Total Tokens", "Cost (USD)", "Latency (ms)", "Status Code", "Error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	offset := 0
	for {
		logs, err := s.usageRepo.GetByTimeRangePaginated(ctx, startTime, endTime, csvBatchSize, offset)
		if err != nil {
			return fmt.Errorf("failed to get usage logs (offset %d): %w", offset, err)
		}
		if len(logs) == 0 {
			break
		}
		for _, log := range logs {
			row := []string{
				log.CreatedAt.Format(time.RFC3339),
				log.ProjectID.String(),
				log.APIKeyID.String(),
				log.ModelName,
				strconv.Itoa(log.RequestTokens),
				strconv.Itoa(log.ResponseTokens),
				strconv.Itoa(log.TotalTokens),
				log.Cost.Round(6).StringFixed(6),
				strconv.FormatInt(log.Latency, 10),
				strconv.Itoa(log.StatusCode),
				log.ErrorMessage,
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
		if len(logs) < csvBatchSize {
			break
		}
		offset += csvBatchSize
	}

	return nil
}

// ─── Helpers ────────────────────────────────────────────────

func statUnitsMoney(units float64) decimal.Decimal {
	return decimal.NewFromFloat(units).
		Div(decimal.NewFromInt(models.MoneyUnitsFactor)).
		Round(4).
		Round(models.MoneyScale)
}

// meanStdDevUnits computes mean and population standard deviation over
// fixed-scale money units. The float boundary is statistics-only.
func meanStdDevUnits(values []int64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range values {
		sum += float64(v)
	}
	mean := sum / float64(len(values))

	var varianceSum float64
	for _, v := range values {
		d := float64(v) - mean
		varianceSum += d * d
	}
	stddev := math.Sqrt(varianceSum / float64(len(values)))

	return mean, stddev
}
