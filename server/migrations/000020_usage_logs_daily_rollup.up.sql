-- usage_logs daily rollup (DB-H1 partial — materialized view portion).
--
-- The dashboard's daily aggregation queries (myDailyUsage, providerStats,
-- modelStats) scan usage_logs with GROUP BY TO_CHAR(created_at, 'YYYY-MM-DD'),
-- which can't use a btree index for the group key. At ~860k rows/day the
-- scan becomes second-scale by ~6 months.
--
-- Add a materialized view that precomputes per-(day, project, channel,
-- provider, model) aggregates. Refreshed concurrently every hour by a
-- background job; dashboard queries hit the view instead of the raw table.
--
-- The full table-partitioning step (PARTITION BY RANGE (created_at)) is
-- intentionally NOT included in this migration — it requires either an
-- offline swap or pg_partman setup and varies per deployment. Operators
-- should follow docs/usage-logs-partitioning.md when the table grows past
-- ~50M rows; this view buys time and is correct either way.

CREATE MATERIALIZED VIEW IF NOT EXISTS usage_logs_daily_rollup AS
SELECT
    date_trunc('day', created_at)::date AS day,
    project_id,
    channel,
    provider_id,
    model_id,
    model_name,
    COUNT(*)                       AS request_count,
    COALESCE(SUM(request_tokens),  0)::bigint AS request_tokens,
    COALESCE(SUM(response_tokens), 0)::bigint AS response_tokens,
    COALESCE(SUM(total_tokens),    0)::bigint AS total_tokens,
    COALESCE(SUM(cost),            0)::double precision AS cost,
    COALESCE(SUM(customer_charge), 0)::double precision AS customer_charge,
    COALESCE(SUM(provider_cost),   0)::double precision AS provider_cost,
    COALESCE(AVG(latency),         0)::bigint AS avg_latency_ms,
    COUNT(*) FILTER (WHERE status_code >= 200 AND status_code < 300) AS success_count,
    COUNT(*) FILTER (WHERE status_code >= 500)                       AS server_error_count
FROM usage_logs
WHERE deleted_at IS NULL
GROUP BY day, project_id, channel, provider_id, model_id, model_name;

-- CONCURRENTLY-eligible: the unique index lets later refreshes run with
-- REFRESH MATERIALIZED VIEW CONCURRENTLY without blocking readers.
CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_pk
    ON usage_logs_daily_rollup (day, project_id, channel, provider_id, model_id, model_name);

-- Query-side indexes covering the common dashboard filters.
CREATE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_project_day
    ON usage_logs_daily_rollup (project_id, day DESC);
CREATE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_project_channel_day
    ON usage_logs_daily_rollup (project_id, channel, day DESC);
