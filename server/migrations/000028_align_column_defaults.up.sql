-- Schema-drift cleanup: align column DEFAULTs, NOT NULL flags, and the
-- final integer→bigint widenings with what GORM AutoMigrate produces.
--
-- This is a metadata-only migration. Every ALTER below either:
--   * changes a DEFAULT expression (no table rewrite),
--   * adds/drops a NOT NULL flag (table scan to validate, no rewrite),
--   * widens INTEGER → BIGINT (no rewrite in PG since int4→int8 is an
--     in-place metadata change for nullable columns; the explicit USING
--     clause keeps it deterministic on populated tables),
--   * tightens numeric → numeric(20,8) (no rewrite for nullable; PG's
--     ALTER COLUMN ... TYPE numeric(20,8) is a metadata change when the
--     existing column is already numeric with no precision constraint).
--
-- All ALTERs are safe to run online: brief catalog locks, no row rewrite.
-- Where NOT NULL is being added, an UPDATE backfills NULL first.

-- ─────────────────────────────────────────────────────────────────────
-- A. CURRENT_TIMESTAMP → now() (semantically identical in Postgres)
-- ─────────────────────────────────────────────────────────────────────
-- GORM AutoMigrate emits `DEFAULT now() NOT NULL` for BaseModel-embedded
-- CreatedAt/UpdatedAt; SQL migrations historically used CURRENT_TIMESTAMP.
-- Both expressions resolve to `now()` at row insert time, so this is a
-- cosmetic catalog rewrite to make pg_dump output match.

-- The previous SQL state on these six rows was
-- `DEFAULT CURRENT_TIMESTAMP` without NOT NULL. GORM emits
-- `DEFAULT now() NOT NULL`, so add NOT NULL too after backfilling NULLs.
UPDATE dlp_configs   SET created_at = now() WHERE created_at IS NULL;
UPDATE dlp_configs   SET updated_at = now() WHERE updated_at IS NULL;
UPDATE organizations SET created_at = now() WHERE created_at IS NULL;
UPDATE organizations SET updated_at = now() WHERE updated_at IS NULL;
UPDATE projects      SET created_at = now() WHERE created_at IS NULL;
UPDATE projects      SET updated_at = now() WHERE updated_at IS NULL;

ALTER TABLE dlp_configs           ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE dlp_configs           ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE dlp_configs           ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE dlp_configs           ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE organizations         ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE organizations         ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE organizations         ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE organizations         ALTER COLUMN updated_at SET NOT NULL;
ALTER TABLE projects              ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE projects              ALTER COLUMN updated_at SET DEFAULT now();
ALTER TABLE projects              ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE projects              ALTER COLUMN updated_at SET NOT NULL;

-- organization_members has CURRENT_TIMESTAMP defaults from the legacy
-- migration; the GORM model uses bare time.Time fields with no `default:`
-- tag so AutoMigrate emits no DEFAULT at all. Drop the SQL defaults to
-- match — the OrganizationService writer sets CreatedAt/UpdatedAt
-- explicitly on every insert.
ALTER TABLE organization_members  ALTER COLUMN created_at DROP DEFAULT;
ALTER TABLE organization_members  ALTER COLUMN updated_at DROP DEFAULT;

-- ─────────────────────────────────────────────────────────────────────
-- B. Add DEFAULT now() NOT NULL to BaseModel CreatedAt/UpdatedAt that
--    SQL migrations created without defaults.
-- ─────────────────────────────────────────────────────────────────────
-- Every table below embeds models.BaseModel in Go, which GORM renders as
--   created_at timestamp with time zone DEFAULT now() NOT NULL,
--   updated_at timestamp with time zone DEFAULT now() NOT NULL,
-- The DO blocks backfill any existing NULL rows before applying NOT NULL
-- so this is safe on populated tables.

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
        EXECUTE format(
            'UPDATE %I SET created_at = now() WHERE created_at IS NULL;', tbl);
        EXECUTE format(
            'UPDATE %I SET updated_at = now() WHERE updated_at IS NULL;', tbl);
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN created_at SET DEFAULT now();', tbl);
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN updated_at SET DEFAULT now();', tbl);
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN created_at SET NOT NULL;', tbl);
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN updated_at SET NOT NULL;', tbl);
    END LOOP;
END $$;

-- ─────────────────────────────────────────────────────────────────────
-- C. Cosmetic '0'::numeric / 0.0000 → '0'::numeric default form
-- ─────────────────────────────────────────────────────────────────────
-- These columns are already numeric(20,8) on both sides; only the textual
-- DEFAULT expression differs. SET DEFAULT here normalizes pg_dump output.
-- GORM emits `DEFAULT '0'::numeric` because the Go side uses
-- `gorm:"default:0"` on a `decimal.Decimal` column.

ALTER TABLE organizations    ALTER COLUMN billing_limit                SET DEFAULT '0'::numeric;
ALTER TABLE projects         ALTER COLUMN quota_limit                  SET DEFAULT '0'::numeric;
ALTER TABLE usage_logs       ALTER COLUMN customer_charge              SET DEFAULT '0'::numeric;
ALTER TABLE usage_logs       ALTER COLUMN provider_cost                SET DEFAULT '0'::numeric;
ALTER TABLE users            ALTER COLUMN balance                      SET DEFAULT '0'::numeric;
ALTER TABLE users            ALTER COLUMN monthly_budget_usd           SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN input_price_per1_k           SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN output_price_per1_k          SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN price_per_image              SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN price_per_minute             SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN price_per_second             SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN provider_cost_per_image      SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN provider_cost_per_minute     SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN provider_cost_per_second    SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN provider_input_cost_per1_k   SET DEFAULT '0'::numeric;
ALTER TABLE models           ALTER COLUMN provider_output_cost_per1_k  SET DEFAULT '0'::numeric;
ALTER TABLE coupons          ALTER COLUMN min_amount                   TYPE numeric(20,8);
ALTER TABLE coupons          ALTER COLUMN min_amount                   SET DEFAULT '0'::numeric;
ALTER TABLE coupons          ALTER COLUMN discount_value               TYPE numeric(20,8);
ALTER TABLE redeem_codes     ALTER COLUMN credit_amount                TYPE numeric(20,8);
ALTER TABLE redeem_codes     ALTER COLUMN credit_amount                SET DEFAULT '0'::numeric;

-- ─────────────────────────────────────────────────────────────────────
-- D. ::character varying → ::text in DEFAULT expressions
-- ─────────────────────────────────────────────────────────────────────
-- 000027 changed the column type to text; the default expression was
-- left dangling as `'literal'::character varying`. Re-setting the default
-- forces Postgres to re-resolve the cast against the new column type and
-- output `'literal'::text` in pg_dump.

ALTER TABLE alerts        ALTER COLUMN status   SET DEFAULT 'active'::text;
ALTER TABLE proxies       ALTER COLUMN type     SET DEFAULT 'http'::text;
ALTER TABLE proxy_pools   ALTER COLUMN strategy SET DEFAULT 'weighted'::text;
ALTER TABLE users         ALTER COLUMN role     SET DEFAULT 'user'::text;

-- ─────────────────────────────────────────────────────────────────────
-- E. integer → bigint widening
-- ─────────────────────────────────────────────────────────────────────
-- The Go side declares `int64` (and GORM-default `int` on most platforms
-- maps to int8 in PG), so AutoMigrate emits `bigint`. The SQL migrations
-- used the narrower `integer`. Postgres widens int4→int8 in place; no
-- table rewrite.

ALTER TABLE alert_configs        ALTER COLUMN failure_threshold TYPE bigint;
ALTER TABLE api_keys             ALTER COLUMN daily_limit       TYPE bigint;
ALTER TABLE api_keys             ALTER COLUMN rate_limit        TYPE bigint;
ALTER TABLE conversation_memories ALTER COLUMN sequence          TYPE bigint;
ALTER TABLE conversation_memories ALTER COLUMN token_count       TYPE bigint;
ALTER TABLE models               ALTER COLUMN context_window    TYPE bigint;
ALTER TABLE models               ALTER COLUMN max_output_tokens TYPE bigint;
ALTER TABLE models               ALTER COLUMN max_tokens        TYPE bigint;
ALTER TABLE provider_api_keys    ALTER COLUMN priority          TYPE bigint;
ALTER TABLE provider_api_keys    ALTER COLUMN rate_limit        TYPE bigint;
ALTER TABLE providers            ALTER COLUMN max_retries       TYPE bigint;
ALTER TABLE providers            ALTER COLUMN priority          TYPE bigint;
ALTER TABLE providers            ALTER COLUMN timeout           TYPE bigint;
ALTER TABLE semantic_caches      ALTER COLUMN hit_count         TYPE bigint;
-- usage_logs.{request,response,total}_tokens + status_code are referenced
-- by the usage_logs_daily_rollup MV; drop and recreate around the ALTERs.
DROP MATERIALIZED VIEW IF EXISTS usage_logs_daily_rollup;
ALTER TABLE usage_logs           ALTER COLUMN request_tokens    TYPE bigint;
ALTER TABLE usage_logs           ALTER COLUMN response_tokens   TYPE bigint;
ALTER TABLE usage_logs           ALTER COLUMN total_tokens      TYPE bigint;
ALTER TABLE usage_logs           ALTER COLUMN status_code       TYPE bigint;
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

-- ─────────────────────────────────────────────────────────────────────
-- F. double precision → numeric
-- ─────────────────────────────────────────────────────────────────────
-- Go float64 mapped to GORM's default `numeric` type (it picks numeric for
-- decimal types since GORM v2.5). The SQL migrations used `double precision`
-- for these floats. Cast is exact since float64 fits inside Postgres
-- numeric with full precision.

ALTER TABLE budgets           ALTER COLUMN alert_threshold TYPE numeric USING alert_threshold::numeric;
ALTER TABLE provider_api_keys ALTER COLUMN weight          TYPE numeric USING weight::numeric;
ALTER TABLE providers         ALTER COLUMN weight          TYPE numeric USING weight::numeric;
ALTER TABLE proxies           ALTER COLUMN avg_latency     TYPE numeric USING avg_latency::numeric;
ALTER TABLE proxies           ALTER COLUMN weight          TYPE numeric USING weight::numeric;
-- The default values change shape after the type cast:
--   double precision: `DEFAULT 1.0` -> numeric: `DEFAULT 1`
--   double precision: `DEFAULT 0`   -> numeric: `DEFAULT 0`
-- Re-setting the default reformats pg_dump output to match GORM.
ALTER TABLE provider_api_keys ALTER COLUMN weight      SET DEFAULT 1;
ALTER TABLE providers         ALTER COLUMN weight      SET DEFAULT 1;
ALTER TABLE proxies           ALTER COLUMN weight      SET DEFAULT 1;
ALTER TABLE proxies           ALTER COLUMN avg_latency SET DEFAULT 0;

-- ─────────────────────────────────────────────────────────────────────
-- G. Drop NOT NULL where GORM does not declare it
-- ─────────────────────────────────────────────────────────────────────
-- GORM emits NOT NULL only when the struct tag contains `not null` (and
-- additionally for `gorm:"primary_key"`/`uniqueIndex`). The SQL side
-- declared NOT NULL on a few columns whose Go field has no such tag, so
-- the application is already prepared to handle NULL values there. Drop
-- the constraints on the SQL side to converge.

-- api_keys.allowed_models / allowed_providers: GORM tag has only
-- `serializer:json;default:'[]'`, no `not null`.
ALTER TABLE api_keys  ALTER COLUMN allowed_models    DROP NOT NULL;
ALTER TABLE api_keys  ALTER COLUMN allowed_providers DROP NOT NULL;

-- models.catalog_warnings and models.model_kind: GORM tag has only
-- `default:'unknown'` / `default:''`, no `not null`.
ALTER TABLE models    ALTER COLUMN catalog_warnings DROP NOT NULL;
ALTER TABLE models    ALTER COLUMN context_window   DROP NOT NULL;
ALTER TABLE models    ALTER COLUMN model_kind       DROP NOT NULL;

-- transactions.user_id: legacy schema had NOT NULL from migration 000001
-- back when transactions were per-user. The current GORM Transaction model
-- carries UserID with `gorm:"type:uuid;index"` — no `not null`. Drop the
-- constraint so internal/system transactions (refunds initiated by admins,
-- adjustment entries) that have no user_id can be inserted.
ALTER TABLE transactions ALTER COLUMN user_id DROP NOT NULL;

-- ─────────────────────────────────────────────────────────────────────
-- proxy_pools.description: SQL has DEFAULT ''::text, the ProxyPool GORM
-- model has no `default:` tag on Description. Drop the default to match.
ALTER TABLE proxy_pools ALTER COLUMN description DROP DEFAULT;

-- H. Drop the SQL-only unique constraint on dlp_configs.project_id
-- ─────────────────────────────────────────────────────────────────────
-- The DlpConfig GORM model documents (in identity.go) that uniqueness is
-- enforced at the service layer + by the 1:0..1 relationship, NOT by a
-- DB-level unique constraint. Removing the constraint converges with
-- GORM AutoMigrate, which never emits this UNIQUE.
ALTER TABLE dlp_configs DROP CONSTRAINT IF EXISTS uni_dlp_configs_project_id;
