// Package quota provides shared monthly usage counters used by the request-time
// quota check (api/middleware) and the post-request usage recorder (service/billing).
//
// The package owns a single source of truth for the Redis hash schema so the
// writer (billing) and reader (middleware) cannot drift.
package quota

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"llm-router-platform/internal/models"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

// ─── Prometheus metrics ─────────────────────────────────────────────────

var (
	// IncrementFailuresTotal counts Redis pipeline errors when recording usage.
	// A spike here means the request-time quota check is reading stale or zero
	// usage — quota checks may pass through unbounded traffic.
	IncrementFailuresTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "billing",
			Name:      "quota_cache_increment_failures_total",
			Help:      "Total Redis failures when incrementing the monthly usage hash.",
		},
	)

	// CheckDBFallbackTotal counts request-time quota checks that fell back to
	// SQL aggregation because Redis was unavailable.
	CheckDBFallbackTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "llm_router",
			Subsystem: "billing",
			Name:      "quota_check_db_fallback_total",
			Help:      "Quota checks served from DB because Redis was unavailable.",
		},
		[]string{"reason"}, // redis_unavailable, redis_timeout
	)
)

const (
	tokensField    = "tokens"
	costUSDField   = "cost_usd"
	costUnitsField = "cost_units"
)

// MonthlyKey returns the Redis hash key that holds the user's current-month
// token + cost counters. Format: quota:<userID>:YYYY-MM.
//
// The middleware quota check reads this key; service/billing writes to it
// after every successful usage record. Both must use this helper.
func MonthlyKey(userID string) string {
	return MonthlyKeyAt(userID, time.Now())
}

// MonthlyKeyAt is MonthlyKey with an explicit timestamp (used by tests).
func MonthlyKeyAt(userID string, t time.Time) string {
	return fmt.Sprintf("quota:%s:%s", userID, t.Format("2006-01"))
}

// IncrementUsageMoney records a token delta and decimal money delta.
//
// Redis only has native integer and floating increments. To keep monthly budget
// checks deterministic, the canonical cached cost is stored as a fixed-scale
// integer in cost_units, where one unit is 10^-8 USD (matching MoneyScale).
// The legacy cost_usd field is read as a fallback for old Redis keys but new
// writes avoid floating increments entirely.
func IncrementUsageMoney(ctx context.Context, rdb *redis.Client, logger *zap.Logger, userID string, tokens int64, costUSD decimal.Decimal) {
	if rdb == nil || userID == "" {
		return
	}

	costUSD = costUSD.Round(models.MoneyScale)
	costUnits := models.MoneyToUnits(costUSD)
	key := MonthlyKey(userID)
	pipe := rdb.Pipeline()
	pipe.HIncrBy(ctx, key, tokensField, tokens)
	pipe.HIncrBy(ctx, key, costUnitsField, costUnits)
	pipe.Expire(ctx, key, 35*24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		IncrementFailuresTotal.Inc()
		if logger != nil {
			logger.Warn("quota cache increment failed",
				zap.String("user_id", userID),
				zap.Int64("tokens", tokens),
				zap.String("cost_usd", costUSD.StringFixed(models.MoneyScale)),
				zap.Int64("cost_units", costUnits),
				zap.Error(err),
			)
		}
	}
}

// ReadMonthlyUsageMoney reads monthly token and decimal cost counters.
func ReadMonthlyUsageMoney(ctx context.Context, rdb *redis.Client, userID string) (int64, decimal.Decimal, error) {
	if rdb == nil {
		return 0, decimal.Zero, nil
	}
	key := MonthlyKey(userID)
	res, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, decimal.Zero, err
	}

	var tokens int64
	if v, ok := res[tokensField]; ok {
		tokens, _ = strconv.ParseInt(v, 10, 64)
	}
	if v, ok := res[costUnitsField]; ok {
		units, parseErr := strconv.ParseInt(v, 10, 64)
		if parseErr == nil {
			return tokens, models.MoneyFromUnits(units), nil
		}
	}
	if v, ok := res[costUSDField]; ok {
		cost, parseErr := decimal.NewFromString(v)
		if parseErr == nil {
			return tokens, cost.Round(models.MoneyScale), nil
		}
	}
	return tokens, decimal.Zero, nil
}
