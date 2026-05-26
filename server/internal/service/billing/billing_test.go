package billing

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"llm-router-platform/internal/models"
	"llm-router-platform/internal/repository"
)

func TestUsageSummary(t *testing.T) {
	summary := UsageSummary{
		TotalRequests: 1000,
		TotalTokens:   50000,
		TotalCost:     models.MoneyFromFloat(25.50),
		AvgLatency:    150.5,
		SuccessRate:   99.5,
		ErrorCount:    5,
	}

	assert.Equal(t, int64(1000), summary.TotalRequests)
	assert.Equal(t, int64(50000), summary.TotalTokens)
	assert.InDelta(t, 25.50, models.MoneyToFloat(summary.TotalCost), 0.01)
	assert.InDelta(t, 99.5, summary.SuccessRate, 0.1)
}

func TestDailyUsage(t *testing.T) {
	daily := DailyUsage{
		Date:     time.Now().Format("2006-01-02"),
		Requests: 100,
		Tokens:   5000,
		Cost:     models.MoneyFromFloat(2.50),
	}

	assert.Equal(t, int64(100), daily.Requests)
	assert.Equal(t, int64(5000), daily.Tokens)
	assert.InDelta(t, 2.50, models.MoneyToFloat(daily.Cost), 0.01)
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

func TestCalculateCustomerChargeIncludesMeteredDimensions(t *testing.T) {
	svc := &Service{}
	model := &models.Model{
		InputPricePer1K:  models.MoneyFromFloat(0.01),
		OutputPricePer1K: models.MoneyFromFloat(0.02),
		PricePerSecond:   models.MoneyFromFloat(0.001),
		PricePerImage:    models.MoneyFromFloat(0.04),
		PricePerMinute:   models.MoneyFromFloat(0.03),
	}
	log := &models.UsageLog{
		RequestTokens:  1000,
		ResponseTokens: 500,
		DurationMs:     90000,
		ItemCount:      2,
	}

	cost := svc.calculateCustomerCharge(model, log)

	assert.InDelta(t, 0.235, models.MoneyToFloat(cost), 0.0001)
}

func TestCalculateProviderCostFallsBackWhenRatesMissing(t *testing.T) {
	svc := &Service{}
	model := &models.Model{}
	log := &models.UsageLog{RequestTokens: 1000}

	cost := svc.calculateProviderCost(model, log, models.MoneyFromFloat(0.42))

	assert.InDelta(t, 0.42, models.MoneyToFloat(cost), 0.0001)
}

func TestCalculateProviderCostUsesProviderRates(t *testing.T) {
	svc := &Service{}
	model := &models.Model{
		ProviderInputCostPer1K:  models.MoneyFromFloat(0.004),
		ProviderOutputCostPer1K: models.MoneyFromFloat(0.008),
		ProviderCostPerImage:    models.MoneyFromFloat(0.02),
	}
	log := &models.UsageLog{
		RequestTokens:  1000,
		ResponseTokens: 500,
		ItemCount:      2,
	}

	cost := svc.calculateProviderCost(model, log, models.MoneyFromFloat(0.50))

	assert.InDelta(t, 0.048, models.MoneyToFloat(cost), 0.0001)
}

func TestUsageLogModel(t *testing.T) {
	log := models.UsageLog{
		ProjectID:      uuid.New(),
		APIKeyID:       uuid.New(),
		ProviderID:     uuid.New(),
		RequestTokens:  100,
		ResponseTokens: 200,
		TotalTokens:    300,
		Cost:           models.MoneyFromFloat(0.01),
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
		{Date: "2024-01-01", Requests: 100, Tokens: 5000, Cost: models.MoneyFromFloat(2.50)},
		{Date: "2024-01-02", Requests: 150, Tokens: 7500, Cost: models.MoneyFromFloat(3.75)},
		{Date: "2024-01-03", Requests: 120, Tokens: 6000, Cost: models.MoneyFromFloat(3.00)},
	}

	var totalRequests, totalTokens int64
	totalCost := models.MoneyFromFloat(0)

	for _, r := range records {
		totalRequests += r.Requests
		totalTokens += r.Tokens
		totalCost = models.MoneyAdd(totalCost, r.Cost)
	}

	assert.Equal(t, int64(370), totalRequests)
	assert.Equal(t, int64(18500), totalTokens)
	assert.InDelta(t, 9.25, models.MoneyToFloat(totalCost), 0.01)
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
	assert.True(t, summary.TotalCost.IsZero())
}

func TestUsageSummaryFromCacheIncludesDashboardFields(t *testing.T) {
	summary, ok := usageSummaryFromCache(map[string]string{
		usageCacheTotalRequests: "4",
		usageCacheTotalTokens:   "120",
		usageCacheTotalCost:     "0.42",
		usageCacheLatencySum:    "500",
		usageCacheSuccessCount:  "3",
		usageCacheErrorCount:    "1",
		usageCacheMCPCallCount:  "7",
		usageCacheMCPErrorCount: "2",
	})

	assert.True(t, ok)
	assert.Equal(t, int64(4), summary.TotalRequests)
	assert.Equal(t, int64(120), summary.TotalTokens)
	assert.InDelta(t, 0.42, models.MoneyToFloat(summary.TotalCost), 0.0001)
	assert.InDelta(t, 125, summary.AvgLatency, 0.0001)
	assert.Equal(t, int64(3), summary.SuccessCount)
	assert.InDelta(t, 75, summary.SuccessRate, 0.0001)
	assert.Equal(t, int64(1), summary.ErrorCount)
	assert.Equal(t, int64(7), summary.MCPCallCount)
	assert.Equal(t, int64(2), summary.MCPErrorCount)
}

func TestUsageSummaryFromCacheRejectsLegacyPartialHash(t *testing.T) {
	_, ok := usageSummaryFromCache(map[string]string{
		usageCacheTotalRequests: "4",
		usageCacheTotalTokens:   "120",
		usageCacheTotalCost:     "0.42",
		"success_rate":          "75",
	})

	assert.False(t, ok)
}

func TestShouldIncrementUsageSummaryCacheSkipsStreamingPrerecord(t *testing.T) {
	assert.False(t, shouldIncrementUsageSummaryCache(&models.UsageLog{StatusCode: http.StatusProcessing}))
	assert.True(t, shouldIncrementUsageSummaryCache(&models.UsageLog{StatusCode: http.StatusOK}))
	assert.True(t, shouldIncrementUsageSummaryCache(&models.UsageLog{StatusCode: 499}))
}

func TestIncrUsageCacheSkipsPrerecordAndTracksDashboardFields(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	orgID := uuid.New()
	projectID := uuid.New()
	require.NoError(t, db.Exec("CREATE TABLE projects (id TEXT PRIMARY KEY, org_id TEXT NOT NULL, deleted_at DATETIME)").Error)
	require.NoError(t, db.Exec("INSERT INTO projects (id, org_id) VALUES (?, ?)", projectID.String(), orgID.String()).Error)

	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	svc := NewService(repository.NewUsageLogRepository(db), nil, rdb, zaptest.NewLogger(t))
	key := usageSummaryCacheKey(orgID, time.Now())

	svc.incrUsageCache(ctx, &models.UsageLog{
		ProjectID:  projectID,
		StatusCode: http.StatusProcessing,
	})
	assert.False(t, mr.Exists(key))

	svc.incrUsageCache(ctx, &models.UsageLog{
		ProjectID:      projectID,
		StatusCode:     http.StatusOK,
		TotalTokens:    42,
		Cost:           models.MoneyFromFloat(0.10),
		CustomerCharge: models.MoneyFromFloat(0.12),
		Latency:        80,
		MCPCallCount:   3,
		MCPErrorCount:  1,
	})

	values, err := rdb.HGetAll(ctx, key).Result()
	require.NoError(t, err)
	assert.Equal(t, "12000000", values[usageCacheTotalCostUnits])
	summary, ok := usageSummaryFromCache(values)
	require.True(t, ok)
	assert.Equal(t, int64(1), summary.TotalRequests)
	assert.Equal(t, int64(42), summary.TotalTokens)
	assert.InDelta(t, 0.12, models.MoneyToFloat(summary.TotalCost), 0.0001)
	assert.InDelta(t, 80, summary.AvgLatency, 0.0001)
	assert.Equal(t, int64(1), summary.SuccessCount)
	assert.Equal(t, int64(0), summary.ErrorCount)
	assert.Equal(t, int64(3), summary.MCPCallCount)
	assert.Equal(t, int64(1), summary.MCPErrorCount)
}

func TestGetUserUsageSummaryFiltersByUserID(t *testing.T) {
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE usage_logs (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			project_id TEXT NOT NULL,
			created_at DATETIME,
			total_tokens INTEGER,
			cost REAL,
			customer_charge REAL,
			latency INTEGER,
			status_code INTEGER,
			mcp_call_count INTEGER,
			mcp_error_count INTEGER,
			deleted_at DATETIME
		)
	`).Error)

	userID := uuid.New()
	otherUserID := uuid.New()
	projectID := uuid.New()
	now := time.Now()
	insertLog := func(id uuid.UUID, tokens int, charge float64, status int) {
		t.Helper()
		require.NoError(t, db.Exec(`
			INSERT INTO usage_logs (
				id, user_id, project_id, created_at, total_tokens, cost,
				customer_charge, latency, status_code, mcp_call_count, mcp_error_count
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, uuid.New().String(), id.String(), projectID.String(), now, tokens, charge, charge, 100, status, 2, 1).Error)
	}
	insertLog(userID, 10, 0.25, http.StatusOK)
	insertLog(userID, 20, 0.75, http.StatusInternalServerError)
	insertLog(otherUserID, 999, 9.99, http.StatusOK)

	svc := NewService(repository.NewUsageLogRepository(db), nil, nil, zaptest.NewLogger(t))
	summary, err := svc.GetUserUsageSummary(ctx, userID, now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, int64(2), summary.TotalRequests)
	assert.Equal(t, int64(30), summary.TotalTokens)
	assert.InDelta(t, 1.0, models.MoneyToFloat(summary.TotalCost), 0.0001)
	assert.Equal(t, int64(1), summary.SuccessCount)
	assert.Equal(t, int64(1), summary.ErrorCount)
	assert.InDelta(t, 50.0, summary.SuccessRate, 0.0001)
	assert.Equal(t, int64(4), summary.MCPCallCount)
	assert.Equal(t, int64(2), summary.MCPErrorCount)
}
