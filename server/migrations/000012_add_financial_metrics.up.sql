-- Split customer-facing charges from provider-side costs so financial
-- reporting can calculate actual gross profit instead of reusing usage cost.

ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS customer_charge DOUBLE PRECISION DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS provider_cost DOUBLE PRECISION DEFAULT 0;

UPDATE usage_logs
SET customer_charge = COALESCE(NULLIF(customer_charge, 0), cost, 0),
    provider_cost = COALESCE(NULLIF(provider_cost, 0), cost, 0)
WHERE customer_charge IS NULL
   OR provider_cost IS NULL
   OR (customer_charge = 0 AND COALESCE(cost, 0) <> 0)
   OR (provider_cost = 0 AND COALESCE(cost, 0) <> 0);

ALTER TABLE models ADD COLUMN IF NOT EXISTS provider_input_cost_per1_k DOUBLE PRECISION DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS provider_output_cost_per1_k DOUBLE PRECISION DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS provider_cost_per_second DOUBLE PRECISION DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS provider_cost_per_image DOUBLE PRECISION DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS provider_cost_per_minute DOUBLE PRECISION DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_logs_customer_charge ON usage_logs(customer_charge);
CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_cost ON usage_logs(provider_cost);
