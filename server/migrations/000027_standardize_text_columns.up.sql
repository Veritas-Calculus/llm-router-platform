-- Schema-drift cleanup: align VARCHAR(N) columns with the TEXT columns that
-- GORM AutoMigrate produces for plain `string` model fields.
--
-- Postgres treats VARCHAR(N) and TEXT essentially identically — VARCHAR's
-- length cap is just an implicit CHECK constraint, not a separate storage
-- format. Switching VARCHAR→TEXT is a metadata-only ALTER (no table
-- rewrite, no row lock beyond the catalog update), so this migration is
-- safe to run online on a populated production database.
--
-- IMPORTANT: any application-level length validation lives in the service
-- layer, not the DB. Columns whose GORM struct tag pins `type:varchar(N)`
-- (e.g. organizations.name, projects.name, dlp_configs.strategy) are
-- intentionally NOT touched here — those keep their VARCHAR width.
--
-- Idempotent: the ALTER ... TYPE is a no-op when the column is already TEXT.

-- alert_configs
ALTER TABLE alert_configs ALTER COLUMN email       TYPE text;
ALTER TABLE alert_configs ALTER COLUMN target_type TYPE text;
ALTER TABLE alert_configs ALTER COLUMN webhook_url TYPE text;

-- alerts
ALTER TABLE alerts ALTER COLUMN alert_type  TYPE text;
ALTER TABLE alerts ALTER COLUMN status      TYPE text;
ALTER TABLE alerts ALTER COLUMN target_type TYPE text;

-- api_keys
ALTER TABLE api_keys ALTER COLUMN key_hash   TYPE text;
ALTER TABLE api_keys ALTER COLUMN key_prefix TYPE text;
ALTER TABLE api_keys ALTER COLUMN name       TYPE text;

-- audit_logs
ALTER TABLE audit_logs ALTER COLUMN action TYPE text;
ALTER TABLE audit_logs ALTER COLUMN ip     TYPE text;

-- budgets
ALTER TABLE budgets ALTER COLUMN email       TYPE text;
ALTER TABLE budgets ALTER COLUMN webhook_url TYPE text;

-- conversation_memories
ALTER TABLE conversation_memories ALTER COLUMN conversation_id TYPE text;
ALTER TABLE conversation_memories ALTER COLUMN role            TYPE text;

-- health_histories
ALTER TABLE health_histories ALTER COLUMN target_type TYPE text;

-- models
ALTER TABLE models ALTER COLUMN display_name TYPE text;
ALTER TABLE models ALTER COLUMN name         TYPE text;

-- provider_api_keys
ALTER TABLE provider_api_keys ALTER COLUMN alias      TYPE text;
ALTER TABLE provider_api_keys ALTER COLUMN key_prefix TYPE text;

-- providers
ALTER TABLE providers ALTER COLUMN base_url TYPE text;
ALTER TABLE providers ALTER COLUMN name     TYPE text;

-- proxies
ALTER TABLE proxies ALTER COLUMN password TYPE text;
ALTER TABLE proxies ALTER COLUMN region   TYPE text;
ALTER TABLE proxies ALTER COLUMN type     TYPE text;
ALTER TABLE proxies ALTER COLUMN url      TYPE text;
ALTER TABLE proxies ALTER COLUMN username TYPE text;

-- proxy_pools
ALTER TABLE proxy_pools ALTER COLUMN name     TYPE text;
ALTER TABLE proxy_pools ALTER COLUMN strategy TYPE text;

-- usage_logs
-- model_name is referenced by the usage_logs_daily_rollup materialized
-- view defined in migration 000022. ALTER ... TYPE on a column under a
-- view or rule is rejected by Postgres, so the MV is dropped and recreated
-- here. The recreate mirrors 000022_money_numeric.up.sql verbatim except
-- for the column type narrowing the view definition does not constrain.
DROP MATERIALIZED VIEW IF EXISTS usage_logs_daily_rollup;
ALTER TABLE usage_logs ALTER COLUMN model_name TYPE text;
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

-- users
ALTER TABLE users ALTER COLUMN email         TYPE text;
ALTER TABLE users ALTER COLUMN name          TYPE text;
ALTER TABLE users ALTER COLUMN password_hash TYPE text;
ALTER TABLE users ALTER COLUMN role          TYPE text;

-- semantic_caches.id was originally VARCHAR(36) holding stringified UUIDs.
-- GORM models it as `uuid.UUID` with `type:uuid;default:gen_random_uuid()`.
-- The USING cast converts existing rows; any row whose id is not a valid
-- 36-char UUID would fail the cast — but the only writer (cache service)
-- has used uuid.NewString() since the table was introduced, so all rows
-- in any environment that has reached production are guaranteed valid.
ALTER TABLE semantic_caches ALTER COLUMN id TYPE uuid USING id::uuid;
ALTER TABLE semantic_caches ALTER COLUMN id SET DEFAULT gen_random_uuid();
