-- Convert money columns from DOUBLE PRECISION / unscaled NUMERIC to
-- explicitly-scaled NUMERIC (DB-C2 in the audit roadmap).
--
-- Pre-migration state was inconsistent:
--
--   transactions.amount             NUMERIC          (no scale → silently truncated to float64 in Go)
--   transactions.balance            NUMERIC
--   orders.amount                   NUMERIC
--   users.balance                   NUMERIC
--   organizations.billing_limit     DECIMAL(20,4)
--   projects.quota_limit            DECIMAL(20,4)
--   usage_logs.cost                 DOUBLE PRECISION (float — known precision loss)
--   usage_logs.customer_charge      DOUBLE PRECISION
--   usage_logs.provider_cost        DOUBLE PRECISION
--   budgets.monthly_limit_usd       DOUBLE PRECISION
--   plans.price_month               NUMERIC
--   models.input_price_per1_k       DOUBLE PRECISION (and the rest of price_per_*)
--
-- Net effect: any sum across users.balance / transactions.amount /
-- usage_logs.customer_charge accumulates float representation error, so
-- a customer's invoice never ties out to the sum of their usage rows.
-- SOC-2 / finance reporting is impossible against this shape.
--
-- This migration converts every money column to NUMERIC(20,8) — wide enough
-- for any conceivable LLM bill, with 8 fractional digits to handle per-token
-- prices like $0.0000015. Existing rows are losslessly cast (NUMERIC handles
-- arbitrary precision; DOUBLE PRECISION values get rounded to 8 digits during
-- the cast, which is already MORE precision than float64 can represent).
--
-- The ALTER TABLE … ALTER COLUMN … TYPE … USING … rewrites each row.
-- Production operators with very large usage_logs should expect this
-- migration to take time proportional to the row count; run during a
-- low-traffic window. The new partial indexes from earlier migrations
-- remain valid (NUMERIC is btree-orderable).

-- ── transactions ──
ALTER TABLE transactions
    ALTER COLUMN amount  TYPE NUMERIC(20,8) USING amount::numeric,
    ALTER COLUMN balance TYPE NUMERIC(20,8) USING balance::numeric;

-- ── orders ──
ALTER TABLE orders
    ALTER COLUMN amount TYPE NUMERIC(20,8) USING amount::numeric;

-- ── users ──
ALTER TABLE users
    ALTER COLUMN balance            TYPE NUMERIC(20,8) USING balance::numeric,
    ALTER COLUMN monthly_budget_usd TYPE NUMERIC(20,8) USING monthly_budget_usd::numeric;

-- ── organizations / projects ──
ALTER TABLE organizations
    ALTER COLUMN billing_limit TYPE NUMERIC(20,8) USING billing_limit::numeric;
ALTER TABLE projects
    ALTER COLUMN quota_limit TYPE NUMERIC(20,8) USING quota_limit::numeric;

-- ── usage_logs ──
ALTER TABLE usage_logs
    ALTER COLUMN cost            TYPE NUMERIC(20,8) USING cost::numeric,
    ALTER COLUMN customer_charge TYPE NUMERIC(20,8) USING customer_charge::numeric,
    ALTER COLUMN provider_cost   TYPE NUMERIC(20,8) USING provider_cost::numeric;

-- ── budgets ──
ALTER TABLE budgets
    ALTER COLUMN monthly_limit_usd TYPE NUMERIC(20,8) USING monthly_limit_usd::numeric;

-- ── plans ──
ALTER TABLE plans
    ALTER COLUMN price_month TYPE NUMERIC(20,8) USING price_month::numeric;

-- ── models ──
ALTER TABLE models
    ALTER COLUMN input_price_per1_k          TYPE NUMERIC(20,8) USING input_price_per1_k::numeric,
    ALTER COLUMN output_price_per1_k         TYPE NUMERIC(20,8) USING output_price_per1_k::numeric,
    ALTER COLUMN provider_input_cost_per1_k  TYPE NUMERIC(20,8) USING provider_input_cost_per1_k::numeric,
    ALTER COLUMN provider_output_cost_per1_k TYPE NUMERIC(20,8) USING provider_output_cost_per1_k::numeric,
    ALTER COLUMN price_per_second            TYPE NUMERIC(20,8) USING price_per_second::numeric,
    ALTER COLUMN price_per_image             TYPE NUMERIC(20,8) USING price_per_image::numeric,
    ALTER COLUMN price_per_minute            TYPE NUMERIC(20,8) USING price_per_minute::numeric,
    ALTER COLUMN provider_cost_per_second    TYPE NUMERIC(20,8) USING provider_cost_per_second::numeric,
    ALTER COLUMN provider_cost_per_image     TYPE NUMERIC(20,8) USING provider_cost_per_image::numeric,
    ALTER COLUMN provider_cost_per_minute    TYPE NUMERIC(20,8) USING provider_cost_per_minute::numeric;

-- The user-facing API still returns these as JSON floats (Go's encoding/json
-- handles float64 directly; the GORM model carries float64 for the
-- transition). A follow-up commit migrates the Go side to
-- shopspring/decimal.Decimal at the boundary, then to Money scalar at the
-- GraphQL layer. This DB-side migration is a strict superset of the
-- float64 precision so it's safe to land independently.
