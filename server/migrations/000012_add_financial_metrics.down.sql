DROP INDEX IF EXISTS idx_usage_logs_provider_cost;
DROP INDEX IF EXISTS idx_usage_logs_customer_charge;

ALTER TABLE models DROP COLUMN IF EXISTS provider_cost_per_minute;
ALTER TABLE models DROP COLUMN IF EXISTS provider_cost_per_image;
ALTER TABLE models DROP COLUMN IF EXISTS provider_cost_per_second;
ALTER TABLE models DROP COLUMN IF EXISTS provider_output_cost_per1_k;
ALTER TABLE models DROP COLUMN IF EXISTS provider_input_cost_per1_k;

ALTER TABLE usage_logs DROP COLUMN IF EXISTS provider_cost;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS customer_charge;
