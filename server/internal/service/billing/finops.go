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

	"llm-router-platform/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ─── Budget Management ──────────────────────────────────────

// Budget represents monthly spending limits for a user or API key.
type Budget struct {
	ID              uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	UserID          uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	APIKeyID        *uuid.UUID `json:"api_key_id,omitempty" gorm:"type:uuid;index"` // Optional: per API key
	MonthlyLimitUSD float64    `json:"monthly_limit_usd" gorm:"not null"`
	AlertThreshold  float64    `json:"alert_threshold" gorm:"default:0.8"` // 0.8 = alert at 80%
	IsActive        bool       `json:"is_active" gorm:"default:true"`
	WebhookURL      string     `json:"webhook_url,omitempty"`
	Email           string     `json:"email,omitempty"`
}

// BudgetStatus represents the current spend vs. budget.
type BudgetStatus struct {
	Budget         *Budget `json:"budget"`
	CurrentSpend   float64 `json:"current_spend"`
	RemainingUSD   float64 `json:"remaining_usd"`
	UsagePercent   float64 `json:"usage_percent"`
	IsOverBudget   bool    `json:"is_over_budget"`
	IsAlertTripped bool    `json:"is_alert_tripped"`
	PeriodStart    string  `json:"period_start"`
	PeriodEnd      string  `json:"period_end"`
}

// BudgetService handles budget creation, checking, and alerting.
type BudgetService struct {
	usageRepo *repository.UsageLogRepository
	logger    *zap.Logger
	budgets   map[uuid.UUID]*Budget // In-memory until DB model integrated
}

// NewBudgetService creates a new budget service.
func NewBudgetService(usageRepo *repository.UsageLogRepository, logger *zap.Logger) *BudgetService {
	return &BudgetService{
		usageRepo: usageRepo,
		logger:    logger,
		budgets:   make(map[uuid.UUID]*Budget),
	}
}

// SetBudget creates or updates a budget for a user.
func (s *BudgetService) SetBudget(userID uuid.UUID, limitUSD, threshold float64, webhookURL, email string) *Budget {
	budget := &Budget{
		ID:              uuid.New(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          userID,
		MonthlyLimitUSD: limitUSD,
		AlertThreshold:  threshold,
		IsActive:        true,
		WebhookURL:      webhookURL,
		Email:           email,
	}
	s.budgets[userID] = budget
	s.logger.Info("budget set",
		zap.String("user_id", userID.String()),
		zap.Float64("limit_usd", limitUSD),
		zap.Float64("threshold", threshold),
	)
	return budget
}

// GetBudget returns the budget for a user.
func (s *BudgetService) GetBudget(userID uuid.UUID) *Budget {
	return s.budgets[userID]
}

// DeleteBudget removes a budget for a user.
func (s *BudgetService) DeleteBudget(userID uuid.UUID) {
	delete(s.budgets, userID)
}

// CheckBudget evaluates current spend vs budget and returns status.
func (s *BudgetService) CheckBudget(ctx context.Context, userID uuid.UUID) (*BudgetStatus, error) {
	budget := s.budgets[userID]
	if budget == nil || !budget.IsActive {
		return nil, nil
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0).Add(-time.Second)

	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage logs: %w", err)
	}

	var currentSpend float64
	for _, log := range logs {
		currentSpend += log.Cost
	}

	usagePercent := 0.0
	if budget.MonthlyLimitUSD > 0 {
		usagePercent = currentSpend / budget.MonthlyLimitUSD
	}

	status := &BudgetStatus{
		Budget:         budget,
		CurrentSpend:   currentSpend,
		RemainingUSD:   math.Max(0, budget.MonthlyLimitUSD-currentSpend),
		UsagePercent:   math.Round(usagePercent*10000) / 100, // 2 decimal places
		IsOverBudget:   currentSpend >= budget.MonthlyLimitUSD,
		IsAlertTripped: usagePercent >= budget.AlertThreshold,
		PeriodStart:    periodStart.Format("2006-01-02"),
		PeriodEnd:      periodEnd.Format("2006-01-02"),
	}

	if status.IsAlertTripped {
		s.logger.Warn("budget alert triggered",
			zap.String("user_id", userID.String()),
			zap.Float64("spend", currentSpend),
			zap.Float64("limit", budget.MonthlyLimitUSD),
			zap.Float64("percent", status.UsagePercent),
		)
	}

	return status, nil
}

// ─── Cost Anomaly Detection ─────────────────────────────────

// AnomalyResult represents the output of anomaly detection.
type AnomalyResult struct {
	IsAnomaly    bool    `json:"is_anomaly"`
	CurrentCost  float64 `json:"current_cost"`
	ExpectedCost float64 `json:"expected_cost"` // Mean of historical
	Deviation    float64 `json:"deviation"`     // Standard deviations from mean
	Threshold    float64 `json:"threshold"`     // σ threshold used
	WindowDays   int     `json:"window_days"`
	Message      string  `json:"message,omitempty"`
}

// DetectCostAnomaly compares today's cost against a sliding window.
// Returns anomaly if current day cost exceeds mean + threshold*σ.
func (s *Service) DetectCostAnomaly(ctx context.Context, userID uuid.UUID, windowDays int, sigmaThreshold float64) (*AnomalyResult, error) {
	if windowDays <= 1 {
		windowDays = 14 // default 14-day window
	}
	if sigmaThreshold <= 0 {
		sigmaThreshold = 3.0 // default 3σ
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	windowStart := todayStart.AddDate(0, 0, -windowDays)

	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, windowStart, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage logs: %w", err)
	}

	// Group costs by day
	dailyCosts := make(map[string]float64)
	var todayCost float64
	todayKey := todayStart.Format("2006-01-02")

	for _, log := range logs {
		dayKey := log.CreatedAt.Format("2006-01-02")
		dailyCosts[dayKey] += log.Cost
		if dayKey == todayKey {
			todayCost += log.Cost
		}
	}

	// Calculate mean and stddev of historical days (excluding today)
	var historicalCosts []float64
	for key, cost := range dailyCosts {
		if key != todayKey {
			historicalCosts = append(historicalCosts, cost)
		}
	}

	// Fill zero-cost days in the window
	for d := 0; d < windowDays; d++ {
		dayKey := todayStart.AddDate(0, 0, -d-1).Format("2006-01-02")
		if _, exists := dailyCosts[dayKey]; !exists {
			historicalCosts = append(historicalCosts, 0)
		}
	}

	if len(historicalCosts) < 3 {
		return &AnomalyResult{
			IsAnomaly:    false,
			CurrentCost:  todayCost,
			ExpectedCost: 0,
			WindowDays:   windowDays,
			Message:      "insufficient data for anomaly detection",
		}, nil
	}

	mean, stddev := meanStdDev(historicalCosts)

	deviation := 0.0
	if stddev > 0 {
		deviation = (todayCost - mean) / stddev
	}

	isAnomaly := deviation > sigmaThreshold

	result := &AnomalyResult{
		IsAnomaly:    isAnomaly,
		CurrentCost:  math.Round(todayCost*10000) / 10000,
		ExpectedCost: math.Round(mean*10000) / 10000,
		Deviation:    math.Round(deviation*100) / 100,
		Threshold:    sigmaThreshold,
		WindowDays:   windowDays,
	}

	if isAnomaly {
		result.Message = fmt.Sprintf("cost anomaly detected: $%.4f is %.1fσ above expected $%.4f", todayCost, deviation, mean)
		s.logger.Warn("cost anomaly detected",
			zap.String("user_id", userID.String()),
			zap.Float64("today_cost", todayCost),
			zap.Float64("expected", mean),
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

	dailyCosts := make(map[string]float64)
	var todayCost float64
	todayKey := todayStart.Format("2006-01-02")

	for _, log := range logs {
		dayKey := log.CreatedAt.Format("2006-01-02")
		dailyCosts[dayKey] += log.Cost
		if dayKey == todayKey {
			todayCost += log.Cost
		}
	}

	var historicalCosts []float64
	for key, cost := range dailyCosts {
		if key != todayKey {
			historicalCosts = append(historicalCosts, cost)
		}
	}

	for d := 0; d < windowDays; d++ {
		dayKey := todayStart.AddDate(0, 0, -d-1).Format("2006-01-02")
		if _, exists := dailyCosts[dayKey]; !exists {
			historicalCosts = append(historicalCosts, 0)
		}
	}

	if len(historicalCosts) < 3 {
		return &AnomalyResult{
			IsAnomaly:   false,
			CurrentCost: todayCost,
			WindowDays:  windowDays,
			Message:     "insufficient data for anomaly detection",
		}, nil
	}

	mean, stddev := meanStdDev(historicalCosts)
	deviation := 0.0
	if stddev > 0 {
		deviation = (todayCost - mean) / stddev
	}

	return &AnomalyResult{
		IsAnomaly:    deviation > sigmaThreshold,
		CurrentCost:  math.Round(todayCost*10000) / 10000,
		ExpectedCost: math.Round(mean*10000) / 10000,
		Deviation:    math.Round(deviation*100) / 100,
		Threshold:    sigmaThreshold,
		WindowDays:   windowDays,
		Message: func() string {
			if deviation > sigmaThreshold {
				return fmt.Sprintf("system cost anomaly: $%.4f is %.1fσ above expected $%.4f", todayCost, deviation, mean)
			}
			return ""
		}(),
	}, nil
}

// ─── CSV Export ──────────────────────────────────────────────

// ExportUsageCSV writes usage logs to a CSV writer.
func (s *Service) ExportUsageCSV(ctx context.Context, userID uuid.UUID, startTime, endTime time.Time, w *csv.Writer) error {
	logs, err := s.usageRepo.GetByUserIDAndTimeRange(ctx, userID, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to get usage logs: %w", err)
	}

	// Write header
	header := []string{
		"Timestamp", "Model", "Input Tokens", "Output Tokens", "Total Tokens",
		"Cost (USD)", "Latency (ms)", "Status Code", "Error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	// Write rows
	for _, log := range logs {
		row := []string{
			log.CreatedAt.Format(time.RFC3339),
			log.ModelName,
			strconv.Itoa(log.RequestTokens),
			strconv.Itoa(log.ResponseTokens),
			strconv.Itoa(log.TotalTokens),
			fmt.Sprintf("%.6f", log.Cost),
			strconv.FormatInt(log.Latency, 10),
			strconv.Itoa(log.StatusCode),
			log.ErrorMessage,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}

// ExportSystemUsageCSV writes system-wide usage to CSV.
func (s *Service) ExportSystemUsageCSV(ctx context.Context, startTime, endTime time.Time, w *csv.Writer) error {
	logs, err := s.usageRepo.GetByTimeRange(ctx, startTime, endTime)
	if err != nil {
		return fmt.Errorf("failed to get usage logs: %w", err)
	}

	header := []string{
		"Timestamp", "User ID", "API Key ID", "Model", "Input Tokens", "Output Tokens",
		"Total Tokens", "Cost (USD)", "Latency (ms)", "Status Code", "Error",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, log := range logs {
		row := []string{
			log.CreatedAt.Format(time.RFC3339),
			log.UserID.String(),
			log.APIKeyID.String(),
			log.ModelName,
			strconv.Itoa(log.RequestTokens),
			strconv.Itoa(log.ResponseTokens),
			strconv.Itoa(log.TotalTokens),
			fmt.Sprintf("%.6f", log.Cost),
			strconv.FormatInt(log.Latency, 10),
			strconv.Itoa(log.StatusCode),
			log.ErrorMessage,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}

// ─── Helpers ────────────────────────────────────────────────

// meanStdDev computes mean and population standard deviation.
func meanStdDev(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	var varianceSum float64
	for _, v := range values {
		d := v - mean
		varianceSum += d * d
	}
	stddev := math.Sqrt(varianceSum / float64(len(values)))

	return mean, stddev
}
