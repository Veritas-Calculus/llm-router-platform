-- Reverse of 000027_standardize_text_columns.up.sql.
--
-- The original VARCHAR length caps were chosen ad hoc across initial
-- migrations (40, 45, 100, 255, 500, etc.). The down path restores a
-- single conservative width per column type — wide enough that no
-- existing row will trip the implicit CHECK constraint. If you need
-- the original exact widths, recreate them column-by-column.

-- semantic_caches.id: uuid → varchar(36)
ALTER TABLE semantic_caches ALTER COLUMN id DROP DEFAULT;
ALTER TABLE semantic_caches ALTER COLUMN id TYPE varchar(36) USING id::text;

-- users
ALTER TABLE users ALTER COLUMN role          TYPE varchar(50);
ALTER TABLE users ALTER COLUMN password_hash TYPE varchar(255);
ALTER TABLE users ALTER COLUMN name          TYPE varchar(255);
ALTER TABLE users ALTER COLUMN email         TYPE varchar(255);

-- usage_logs
-- model_name is referenced by the usage_logs_daily_rollup materialized
-- view (see the matching .up.sql for context).
DROP MATERIALIZED VIEW IF EXISTS usage_logs_daily_rollup;
ALTER TABLE usage_logs ALTER COLUMN model_name TYPE varchar(255);
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
    COALESCE(SUM(cost),            0)::numeric(20,8) AS cost,
    COALESCE(SUM(customer_charge), 0)::numeric(20,8) AS customer_charge,
    COALESCE(SUM(provider_cost),   0)::numeric(20,8) AS provider_cost,
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

-- proxy_pools
ALTER TABLE proxy_pools ALTER COLUMN strategy TYPE varchar(32);
ALTER TABLE proxy_pools ALTER COLUMN name     TYPE varchar(255);

-- proxies
ALTER TABLE proxies ALTER COLUMN username TYPE varchar(255);
ALTER TABLE proxies ALTER COLUMN url      TYPE varchar(500);
ALTER TABLE proxies ALTER COLUMN type     TYPE varchar(20);
ALTER TABLE proxies ALTER COLUMN region   TYPE varchar(100);
ALTER TABLE proxies ALTER COLUMN password TYPE varchar(500);

-- providers
ALTER TABLE providers ALTER COLUMN name     TYPE varchar(255);
ALTER TABLE providers ALTER COLUMN base_url TYPE varchar(500);

-- provider_api_keys
ALTER TABLE provider_api_keys ALTER COLUMN key_prefix TYPE varchar(20);
ALTER TABLE provider_api_keys ALTER COLUMN alias      TYPE varchar(255);

-- models
ALTER TABLE models ALTER COLUMN name         TYPE varchar(255);
ALTER TABLE models ALTER COLUMN display_name TYPE varchar(255);

-- health_histories
ALTER TABLE health_histories ALTER COLUMN target_type TYPE varchar(50);

-- conversation_memories
ALTER TABLE conversation_memories ALTER COLUMN role            TYPE varchar(50);
ALTER TABLE conversation_memories ALTER COLUMN conversation_id TYPE varchar(255);

-- budgets
ALTER TABLE budgets ALTER COLUMN webhook_url TYPE varchar(500);
ALTER TABLE budgets ALTER COLUMN email       TYPE varchar(255);

-- audit_logs
ALTER TABLE audit_logs ALTER COLUMN ip     TYPE varchar(45);
ALTER TABLE audit_logs ALTER COLUMN action TYPE varchar(100);

-- api_keys
ALTER TABLE api_keys ALTER COLUMN name       TYPE varchar(255);
ALTER TABLE api_keys ALTER COLUMN key_prefix TYPE varchar(20);
ALTER TABLE api_keys ALTER COLUMN key_hash   TYPE varchar(255);

-- alerts
ALTER TABLE alerts ALTER COLUMN target_type TYPE varchar(50);
ALTER TABLE alerts ALTER COLUMN status      TYPE varchar(20);
ALTER TABLE alerts ALTER COLUMN alert_type  TYPE varchar(50);

-- alert_configs
ALTER TABLE alert_configs ALTER COLUMN webhook_url TYPE varchar(500);
ALTER TABLE alert_configs ALTER COLUMN target_type TYPE varchar(50);
ALTER TABLE alert_configs ALTER COLUMN email       TYPE varchar(255);
