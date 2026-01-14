package billing

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"llm-router-platform/internal/models"
)

func TestUsageSummary(t *testing.T) {
	summary := UsageSummary{
		TotalRequests: 1000,
		TotalTokens:   50000,
		TotalCost:     25.50,
		AvgLatency:    150.5,
		SuccessRate:   99.5,
		ErrorCount:    5,
	}

	assert.Equal(t, int64(1000), summary.TotalRequests)
	assert.Equal(t, int64(50000), summary.TotalTokens)
	assert.InDelta(t, 25.50, summary.TotalCost, 0.01)
	assert.InDelta(t, 99.5, summary.SuccessRate, 0.1)
}

func TestDailyUsage(t *testing.T) {
	daily := DailyUsage{
		Date:     time.Now().Format("2006-01-02"),
		Requests: 100,
		Tokens:   5000,
		Cost:     2.50,
	}

	assert.Equal(t, int64(100), daily.Requests)
	assert.Equal(t, int64(5000), daily.Tokens)
	assert.InDelta(t, 2.50, daily.Cost, 0.01)
}

func TestCostCalculation(t *testing.T) {
	inputTokens := 1000
	outputTokens := 2000
	inputPrice := 0.03 / 1000
	outputPrice := 0.06 / 1000

	inputCost := float64(inputTokens) * inputPrice
	outputCost := float64(outputTokens) * outputPrice
	totalCost := inputCost + outputCost

	assert.InDelta(t, 0.03, inputCost, 0.001)
	assert.InDelta(t, 0.12, outputCost, 0.001)
	assert.InDelta(t, 0.15, totalCost, 0.001)
}

func TestUsageLogModel(t *testing.T) {
	log := models.UsageLog{
		UserID:         uuid.New(),
		APIKeyID:       uuid.New(),
		ProviderID:     uuid.New(),
		RequestTokens:  100,
		ResponseTokens: 200,
		TotalTokens:    300,
		Cost:           0.01,
		Latency:        500,
		StatusCode:     200,
	}

	assert.Equal(t, 100, log.RequestTokens)
	assert.Equal(t, 200, log.ResponseTokens)
	assert.Equal(t, 300, log.TotalTokens)
	assert.Equal(t, 200, log.StatusCode)
}

func TestUsageAggregation(t *testing.T) {
	records := []DailyUsage{
		{Date: "2024-01-01", Requests: 100, Tokens: 5000, Cost: 2.50},
		{Date: "2024-01-02", Requests: 150, Tokens: 7500, Cost: 3.75},
		{Date: "2024-01-03", Requests: 120, Tokens: 6000, Cost: 3.00},
	}

	var totalRequests, totalTokens int64
	var totalCost float64

	for _, r := range records {
		totalRequests += r.Requests
		totalTokens += r.Tokens
		totalCost += r.Cost
	}

	assert.Equal(t, int64(370), totalRequests)
	assert.Equal(t, int64(18500), totalTokens)
	assert.InDelta(t, 9.25, totalCost, 0.01)
}

func TestSuccessRateCalculation(t *testing.T) {
	totalRequests := int64(1000)
	successCount := int64(990)
	errorCount := int64(10)

	successRate := float64(successCount) / float64(totalRequests) * 100

	assert.InDelta(t, 99.0, successRate, 0.1)
	assert.Equal(t, totalRequests, successCount+errorCount)
}

func TestAverageLatencyCalculation(t *testing.T) {
	latencies := []int64{100, 150, 200, 120, 180}

	var sum int64
	for _, l := range latencies {
		sum += l
	}
	avg := float64(sum) / float64(len(latencies))

	assert.InDelta(t, 150.0, avg, 0.1)
}

func TestTimePeriods(t *testing.T) {
	now := time.Now()

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	assert.True(t, startOfDay.Before(now) || startOfDay.Equal(now))
	assert.True(t, startOfMonth.Before(now) || startOfMonth.Equal(now))
	assert.True(t, thirtyDaysAgo.Before(now))
}

func TestEmptyUsageSummary(t *testing.T) {
	summary := UsageSummary{}

	assert.Equal(t, int64(0), summary.TotalRequests)
	assert.Equal(t, int64(0), summary.TotalTokens)
	assert.Equal(t, float64(0), summary.TotalCost)
}
