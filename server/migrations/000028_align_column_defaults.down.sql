-- Reverse of 000028_align_column_defaults.up.sql.
--
-- Notes:
--   * The original CURRENT_TIMESTAMP default expression is restored.
--     PG normalizes `CURRENT_TIMESTAMP` and `now()` to the same internal
--     form for execution, so functional equivalence is preserved.
--   * Integer columns revert to `integer` width. This narrowing only
--     succeeds when no row holds a value > 2^31 - 1.
--   * Numeric columns originally typed `numeric` (no precision constraint)
--     come back as plain `numeric` after `coupons`/`redeem_codes` revert.

-- H. Restore the unique constraint on dlp_configs.project_id
ALTER TABLE dlp_configs
    ADD CONSTRAINT uni_dlp_configs_project_id UNIQUE (project_id);

-- proxy_pools.description: restore the empty-string default
ALTER TABLE proxy_pools ALTER COLUMN description SET DEFAULT ''::text;

-- G. Restore NOT NULL constraints
ALTER TABLE transactions ALTER COLUMN user_id          SET NOT NULL;
ALTER TABLE models       ALTER COLUMN model_kind       SET NOT NULL;
ALTER TABLE models       ALTER COLUMN context_window   SET NOT NULL;
ALTER TABLE models       ALTER COLUMN catalog_warnings SET NOT NULL;
ALTER TABLE api_keys     ALTER COLUMN allowed_providers SET NOT NULL;
ALTER TABLE api_keys     ALTER COLUMN allowed_models    SET NOT NULL;

-- F. numeric → double precision (lossy in theory; in practice all values
-- written by the GORM model originated from float64 and round-trip cleanly).
ALTER TABLE proxies           ALTER COLUMN avg_latency DROP DEFAULT;
ALTER TABLE proxies           ALTER COLUMN weight      DROP DEFAULT;
ALTER TABLE providers         ALTER COLUMN weight      DROP DEFAULT;
ALTER TABLE provider_api_keys ALTER COLUMN weight      DROP DEFAULT;
ALTER TABLE proxies           ALTER COLUMN weight          TYPE double precision USING weight::double precision;
ALTER TABLE proxies           ALTER COLUMN avg_latency     TYPE double precision USING avg_latency::double precision;
ALTER TABLE providers         ALTER COLUMN weight          TYPE double precision USING weight::double precision;
ALTER TABLE provider_api_keys ALTER COLUMN weight          TYPE double precision USING weight::double precision;
ALTER TABLE budgets           ALTER COLUMN alert_threshold TYPE double precision USING alert_threshold::double precision;
ALTER TABLE proxies           ALTER COLUMN avg_latency SET DEFAULT 0;
ALTER TABLE proxies           ALTER COLUMN weight      SET DEFAULT 1.0;
ALTER TABLE providers         ALTER COLUMN weight      SET DEFAULT 1.0;
ALTER TABLE provider_api_keys ALTER COLUMN weight      SET DEFAULT 1.0;
ALTER TABLE budgets           ALTER COLUMN alert_threshold SET DEFAULT 0.8;

-- E. bigint → integer narrowing + MV recreate.
DROP MATERIALIZED VIEW IF EXISTS usage_logs_daily_rollup;
ALTER TABLE usage_logs            ALTER COLUMN status_code       TYPE integer;
ALTER TABLE usage_logs            ALTER COLUMN total_tokens      TYPE integer;
ALTER TABLE usage_logs            ALTER COLUMN response_tokens   TYPE integer;
ALTER TABLE usage_logs            ALTER COLUMN request_tokens    TYPE integer;
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

ALTER TABLE semantic_caches       ALTER COLUMN hit_count         TYPE integer;
ALTER TABLE providers             ALTER COLUMN timeout           TYPE integer;
ALTER TABLE providers             ALTER COLUMN priority          TYPE integer;
ALTER TABLE providers             ALTER COLUMN max_retries       TYPE integer;
ALTER TABLE provider_api_keys     ALTER COLUMN rate_limit        TYPE integer;
ALTER TABLE provider_api_keys     ALTER COLUMN priority          TYPE integer;
ALTER TABLE models                ALTER COLUMN max_tokens        TYPE integer;
ALTER TABLE models                ALTER COLUMN max_output_tokens TYPE integer;
ALTER TABLE models                ALTER COLUMN context_window    TYPE integer;
ALTER TABLE conversation_memories ALTER COLUMN token_count       TYPE integer;
ALTER TABLE conversation_memories ALTER COLUMN sequence          TYPE integer;
ALTER TABLE api_keys              ALTER COLUMN rate_limit        TYPE integer;
ALTER TABLE api_keys              ALTER COLUMN daily_limit       TYPE integer;
ALTER TABLE alert_configs         ALTER COLUMN failure_threshold TYPE integer;

-- D. text default casts → character varying
ALTER TABLE users         ALTER COLUMN role     SET DEFAULT 'user'::character varying;
ALTER TABLE proxy_pools   ALTER COLUMN strategy SET DEFAULT 'weighted'::character varying;
ALTER TABLE proxies       ALTER COLUMN type     SET DEFAULT 'http'::character varying;
ALTER TABLE alerts        ALTER COLUMN status   SET DEFAULT 'active'::character varying;

-- C. '0'::numeric → 0 / 0.0000 defaults
ALTER TABLE redeem_codes ALTER COLUMN credit_amount     SET DEFAULT 0;
ALTER TABLE redeem_codes ALTER COLUMN credit_amount     TYPE numeric;
ALTER TABLE coupons      ALTER COLUMN discount_value    TYPE numeric;
ALTER TABLE coupons      ALTER COLUMN min_amount        SET DEFAULT 0;
ALTER TABLE coupons      ALTER COLUMN min_amount        TYPE numeric;
ALTER TABLE models       ALTER COLUMN provider_output_cost_per1_k SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN provider_input_cost_per1_k  SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN provider_cost_per_second   SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN provider_cost_per_minute    SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN provider_cost_per_image     SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN price_per_second            SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN price_per_minute            SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN price_per_image             SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN output_price_per1_k         SET DEFAULT 0;
ALTER TABLE models       ALTER COLUMN input_price_per1_k          SET DEFAULT 0;
ALTER TABLE users        ALTER COLUMN monthly_budget_usd          SET DEFAULT 0;
ALTER TABLE users        ALTER COLUMN balance                     SET DEFAULT 0;
ALTER TABLE usage_logs   ALTER COLUMN provider_cost               SET DEFAULT 0;
ALTER TABLE usage_logs   ALTER COLUMN customer_charge             SET DEFAULT 0;
ALTER TABLE projects     ALTER COLUMN quota_limit                 SET DEFAULT 0.0000;
ALTER TABLE organizations ALTER COLUMN billing_limit              SET DEFAULT 0.0000;

-- B. Drop DEFAULT now() NOT NULL from BaseModel timestamps
DO $$
DECLARE
    tbl text;
BEGIN
    FOR tbl IN SELECT unnest(ARRAY[
        'announcements',
        'async_tasks',
        'cache_configs',
        'coupons',
        'documents',
        'identity_providers',
        'invite_codes',
        'mcp_servers',
        'mcp_tools',
        'notification_channels',
        'orders',
        'plans',
        'prompt_templates',
        'prompt_versions',
        'redeem_codes',
        'subscriptions',
        'system_configs',
        'transactions',
        'webhook_deliveries',
        'webhook_endpoints'
    ]) LOOP
        EXECUTE format('ALTER TABLE %I ALTER COLUMN updated_at DROP NOT NULL;', tbl);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN created_at DROP NOT NULL;', tbl);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN updated_at DROP DEFAULT;', tbl);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN created_at DROP DEFAULT;', tbl);
    END LOOP;
END $$;

-- A. now() → CURRENT_TIMESTAMP + drop NOT NULL added in up
ALTER TABLE projects             ALTER COLUMN updated_at DROP NOT NULL;
ALTER TABLE projects             ALTER COLUMN created_at DROP NOT NULL;
ALTER TABLE organizations        ALTER COLUMN updated_at DROP NOT NULL;
ALTER TABLE organizations        ALTER COLUMN created_at DROP NOT NULL;
ALTER TABLE dlp_configs          ALTER COLUMN updated_at DROP NOT NULL;
ALTER TABLE dlp_configs          ALTER COLUMN created_at DROP NOT NULL;

ALTER TABLE organization_members ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE organization_members ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE projects             ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE projects             ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE organizations        ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE organizations        ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE dlp_configs          ALTER COLUMN created_at SET DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE dlp_configs          ALTER COLUMN updated_at SET DEFAULT CURRENT_TIMESTAMP;
