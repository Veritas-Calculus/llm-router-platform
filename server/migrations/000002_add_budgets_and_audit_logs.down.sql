-- 000002_add_budgets_and_audit_logs.down.sql
-- Rollback budget and audit log tables

DROP TABLE IF EXISTS budgets;
DROP TABLE IF EXISTS audit_logs;

ALTER TABLE users DROP COLUMN IF EXISTS require_password_change;
ALTER TABLE users DROP COLUMN IF EXISTS monthly_token_limit;
ALTER TABLE users DROP COLUMN IF EXISTS monthly_budget_usd;
ALTER TABLE users DROP COLUMN IF EXISTS tokens_invalidated_at;

ALTER TABLE provider_api_keys DROP COLUMN IF EXISTS priority;
