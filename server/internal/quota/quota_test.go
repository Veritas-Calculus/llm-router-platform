package quota

import (
	"context"
	"testing"
	"time"

	"llm-router-platform/internal/models"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb, mr
}

func TestIncrementUsageWritesHash(t *testing.T) {
	rdb, mr := newTestRedis(t)
	ctx := context.Background()
	logger := zap.NewNop()

	IncrementUsageMoney(ctx, rdb, logger, "user-1", 100, models.MoneyFromFloat(0.0125))

	key := MonthlyKey("user-1")
	assert.True(t, mr.Exists(key), "expected hash to exist at %s", key)

	assert.Equal(t, "1250000", mr.HGet(key, costUnitsField))

	tokens, moneyCost, err := ReadMonthlyUsageMoney(ctx, rdb, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(100), tokens)
	assert.True(t, moneyCost.Equal(models.MoneyFromFloat(0.0125)))

	// Second increment accumulates.
	IncrementUsageMoney(ctx, rdb, logger, "user-1", 50, models.MoneyFromFloat(0.005))
	tokens, moneyCost, err = ReadMonthlyUsageMoney(ctx, rdb, "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(150), tokens)
	assert.True(t, moneyCost.Equal(models.MoneyFromFloat(0.0175)))
}

func TestIncrementUsageSetsTTL(t *testing.T) {
	rdb, mr := newTestRedis(t)
	ctx := context.Background()
	logger := zap.NewNop()

	IncrementUsageMoney(ctx, rdb, logger, "user-2", 10, models.MoneyFromFloat(0.01))

	key := MonthlyKey("user-2")
	ttl := mr.TTL(key)
	assert.Greater(t, ttl, 30*24*time.Hour, "TTL should be ~35 days, got %s", ttl)
	assert.LessOrEqual(t, ttl, 35*24*time.Hour+time.Second, "TTL should not exceed 35 days")
}

func TestIncrementUsageNoOpOnNilClient(t *testing.T) {
	IncrementUsageMoney(context.Background(), nil, zap.NewNop(), "anyone", 1, models.MoneyFromFloat(1))
}

func TestIncrementUsageNoOpOnEmptyUserID(t *testing.T) {
	rdb, mr := newTestRedis(t)
	IncrementUsageMoney(context.Background(), rdb, zap.NewNop(), "", 1, models.MoneyFromFloat(1))
	assert.Empty(t, mr.Keys(), "expected no keys to be written for empty userID")
}

func TestReadMonthlyUsageMissingHash(t *testing.T) {
	rdb, _ := newTestRedis(t)
	tokens, cost, err := ReadMonthlyUsageMoney(context.Background(), rdb, "ghost-user")
	require.NoError(t, err)
	assert.Zero(t, tokens)
	assert.True(t, cost.IsZero())
}

func TestMonthlyKeyFormat(t *testing.T) {
	t1 := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	got := MonthlyKeyAt("user-x", t1)
	assert.Equal(t, "quota:user-x:2026-05", got)
}

func TestIncrementUsageRecordsFailure(t *testing.T) {
	rdb, mr := newTestRedis(t)
	mr.Close() // force connection failures
	defer func() { _ = rdb.Close() }()

	before := testutil.ToFloat64(IncrementFailuresTotal)
	IncrementUsageMoney(context.Background(), rdb, zap.NewNop(), "user-x", 10, models.MoneyFromFloat(0.01))
	after := testutil.ToFloat64(IncrementFailuresTotal)
	assert.Greater(t, after, before, "expected failure counter to increment when Redis is unreachable")
}
