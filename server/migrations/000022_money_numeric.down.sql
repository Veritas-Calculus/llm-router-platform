-- WARNING: reverting NUMERIC(20,8) back to DOUBLE PRECISION loses precision.
-- This down migration is provided for symmetry but operators should not run
-- it against production data — by the time billing rows exist, the precision
-- they carry exceeds what float64 can represent. The cast will silently
-- round.

DROP MATERIALIZED VIEW IF EXISTS usage_logs_daily_rollup;

ALTER TABLE models
    ALTER COLUMN input_price_per1_k          TYPE DOUBLE PRECISION USING input_price_per1_k::double precision,
    ALTER COLUMN output_price_per1_k         TYPE DOUBLE PRECISION USING output_price_per1_k::double precision,
    ALTER COLUMN provider_input_cost_per1_k  TYPE DOUBLE PRECISION USING provider_input_cost_per1_k::double precision,
    ALTER COLUMN provider_output_cost_per1_k TYPE DOUBLE PRECISION USING provider_output_cost_per1_k::double precision,
    ALTER COLUMN price_per_second            TYPE NUMERIC USING price_per_second::numeric,
    ALTER COLUMN price_per_image             TYPE NUMERIC USING price_per_image::numeric,
    ALTER COLUMN price_per_minute            TYPE NUMERIC USING price_per_minute::numeric,
    ALTER COLUMN provider_cost_per_second    TYPE DOUBLE PRECISION USING provider_cost_per_second::double precision,
    ALTER COLUMN provider_cost_per_image     TYPE DOUBLE PRECISION USING provider_cost_per_image::double precision,
    ALTER COLUMN provider_cost_per_minute    TYPE DOUBLE PRECISION USING provider_cost_per_minute::double precision;

ALTER TABLE plans
    ALTER COLUMN price_month TYPE NUMERIC USING price_month::numeric;

ALTER TABLE budgets
    ALTER COLUMN monthly_limit_usd TYPE DOUBLE PRECISION USING monthly_limit_usd::double precision;

ALTER TABLE usage_logs
    ALTER COLUMN cost            TYPE DOUBLE PRECISION USING cost::double precision,
    ALTER COLUMN customer_charge TYPE DOUBLE PRECISION USING customer_charge::double precision,
    ALTER COLUMN provider_cost   TYPE DOUBLE PRECISION USING provider_cost::double precision;

ALTER TABLE projects
    ALTER COLUMN quota_limit TYPE NUMERIC USING quota_limit::numeric;
ALTER TABLE organizations
    ALTER COLUMN billing_limit TYPE NUMERIC USING billing_limit::numeric;

ALTER TABLE users
    ALTER COLUMN balance            TYPE NUMERIC USING balance::numeric,
    ALTER COLUMN monthly_budget_usd TYPE DOUBLE PRECISION USING monthly_budget_usd::double precision;

ALTER TABLE orders
    ALTER COLUMN amount TYPE NUMERIC USING amount::numeric;

ALTER TABLE transactions
    ALTER COLUMN amount  TYPE NUMERIC USING amount::numeric,
    ALTER COLUMN balance TYPE NUMERIC USING balance::numeric;

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

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_pk
    ON usage_logs_daily_rollup (day, project_id, channel, provider_id, model_id, model_name);

CREATE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_project_day
    ON usage_logs_daily_rollup (project_id, day DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_daily_rollup_project_channel_day
    ON usage_logs_daily_rollup (project_id, channel, day DESC);
