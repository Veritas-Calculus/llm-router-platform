package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"llm-router-platform/internal/quota"
)

func newQuotaTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

func setupQuotaRouter(t *testing.T, q *QuotaChecker, userID string, tokenLimit int64, budgetLimit float64) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		if tokenLimit > 0 {
			c.Set("user_monthly_token_limit", tokenLimit)
		}
		if budgetLimit > 0 {
			c.Set("user_monthly_budget_usd", budgetLimit)
		}
		c.Next()
	})
	r.Use(q.Check())
	r.GET("/v1/test", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func TestQuotaCheckPassesWithNoUsage(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	q := NewQuotaChecker(rdb, zap.NewNop())
	r := setupQuotaRouter(t, q, "user-1", 1000, 0)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQuotaCheckBlocksWhenTokenLimitExceeded(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	logger := zap.NewNop()

	// Pre-populate the quota hash so the request appears over its limit.
	quota.IncrementUsage(context.Background(), rdb, logger, "user-2", 10_000, 0.50)

	q := NewQuotaChecker(rdb, logger)
	r := setupQuotaRouter(t, q, "user-2", 5_000, 0)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "monthly token quota exceeded")
}

func TestQuotaCheckBlocksWhenBudgetLimitExceeded(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	logger := zap.NewNop()

	quota.IncrementUsage(context.Background(), rdb, logger, "user-3", 100, 5.00)

	q := NewQuotaChecker(rdb, logger)
	r := setupQuotaRouter(t, q, "user-3", 0, 1.00)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "monthly budget quota exceeded")
}

func TestQuotaCheckSetsRemainingHeaders(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	logger := zap.NewNop()

	quota.IncrementUsage(context.Background(), rdb, logger, "user-4", 1_000, 0.10)

	q := NewQuotaChecker(rdb, logger)
	r := setupQuotaRouter(t, q, "user-4", 5_000, 1.00)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "5000", w.Header().Get("X-Quota-Tokens-Limit"))
	assert.Equal(t, "4000", w.Header().Get("X-Quota-Tokens-Remaining"))
	assert.Equal(t, "1.00", w.Header().Get("X-Quota-Budget-Limit"))
	assert.Equal(t, "0.90", w.Header().Get("X-Quota-Budget-Remaining"))
}

func TestQuotaCheckEnforcesBudgetHardLimit(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	logger := zap.NewNop()

	q := NewQuotaChecker(rdb, logger)
	q.WithBudget(func(ctx context.Context, userID uuid.UUID) (bool, bool, float64, float64, error) {
		// User has a $10 budget with hard-limit ON, current spend $12.
		return true, true, 10.0, 12.0, nil
	})

	r := setupQuotaRouter(t, q, uuid.NewString(), 0, 0)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "monthly budget hard limit exceeded")
}

func TestQuotaCheckSkipsBudgetWhenHardLimitOff(t *testing.T) {
	rdb, _ := newQuotaTestRedis(t)
	logger := zap.NewNop()

	q := NewQuotaChecker(rdb, logger)
	q.WithBudget(func(ctx context.Context, userID uuid.UUID) (bool, bool, float64, float64, error) {
		// Soft alert tripped, but hard limit OFF — should not block.
		return false, true, 10.0, 12.0, nil
	})

	r := setupQuotaRouter(t, q, uuid.NewString(), 0, 0)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestQuotaCheckRedisFailureFallsThrough(t *testing.T) {
	rdb, mr := newQuotaTestRedis(t)
	mr.Close() // simulate Redis outage

	q := NewQuotaChecker(rdb, zap.NewNop())
	r := setupQuotaRouter(t, q, "user-5", 5_000, 1.00)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/v1/test", nil)
	r.ServeHTTP(w, req)

	// Redis is down, but no soft limit was hit (we treat 0/0 used) and no
	// hard-limit budget is wired in this test. The request must complete.
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIncrementUsageBackwardCompatibleWrapper(t *testing.T) {
	rdb, mr := newQuotaTestRedis(t)
	IncrementUsage(rdb, "user-x", 42, 0.05)
	assert.True(t, mr.Exists(quota.MonthlyKey("user-x")))
}
