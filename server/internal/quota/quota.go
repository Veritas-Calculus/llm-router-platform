// Package quota provides shared monthly usage counters used by the request-time
// quota check (api/middleware) and the post-request usage recorder (service/billing).
//
// The package owns a single source of truth for the Redis hash schema so the
// writer (billing) and reader (middleware) cannot drift.
package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
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

// IncrementUsage records a delta of tokens and cost to the user's monthly
// hash. Errors are logged + counted but never returned: the calling path is
// always after a successful DB write, and we don't want to fail the request
// (or roll back the DB transaction) just because Redis blipped.
//
// The hash is set to expire 35 days after the latest write so a stale month
// can never serve as input to the next month's quota check.
func IncrementUsage(ctx context.Context, rdb *redis.Client, logger *zap.Logger, userID string, tokens int64, costUSD float64) {
	if rdb == nil || userID == "" {
		return
	}

	key := MonthlyKey(userID)
	pipe := rdb.Pipeline()
	pipe.HIncrBy(ctx, key, "tokens", tokens)
	pipe.HIncrByFloat(ctx, key, "cost_usd", costUSD)
	pipe.Expire(ctx, key, 35*24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		IncrementFailuresTotal.Inc()
		if logger != nil {
			logger.Warn("quota cache increment failed",
				zap.String("user_id", userID),
				zap.Int64("tokens", tokens),
				zap.Float64("cost_usd", costUSD),
				zap.Error(err),
			)
		}
	}
}

// ReadMonthlyUsage reads the current usage counters for a user. Returns
// (tokens, costUSD, error). Caller decides whether to fail open or fall back.
func ReadMonthlyUsage(ctx context.Context, rdb *redis.Client, userID string) (int64, float64, error) {
	if rdb == nil {
		return 0, 0, nil
	}
	key := MonthlyKey(userID)
	res, err := rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return 0, 0, err
	}

	var tokens int64
	var cost float64
	if v, ok := res["tokens"]; ok {
		_, _ = fmt.Sscanf(v, "%d", &tokens)
	}
	if v, ok := res["cost_usd"]; ok {
		_, _ = fmt.Sscanf(v, "%f", &cost)
	}
	return tokens, cost, nil
}
