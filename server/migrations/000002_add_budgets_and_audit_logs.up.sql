-- 000002_add_budgets_and_audit_logs.up.sql
-- Adds budget management and audit logging tables

-- Budgets table (replaces in-memory budget storage)
CREATE TABLE IF NOT EXISTS budgets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL,
    api_key_id UUID,
    monthly_limit_usd DOUBLE PRECISION NOT NULL,
    alert_threshold DOUBLE PRECISION DEFAULT 0.8,
    is_active BOOLEAN DEFAULT true,
    webhook_url VARCHAR(500),
    email VARCHAR(255)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_user_id ON budgets(user_id);
CREATE INDEX IF NOT EXISTS idx_budgets_api_key_id ON budgets(api_key_id);
CREATE INDEX IF NOT EXISTS idx_budgets_deleted_at ON budgets(deleted_at);

-- Audit Logs table (security event recording)
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action VARCHAR(100) NOT NULL,
    actor_id UUID,
    target_id UUID,
    ip VARCHAR(45),
    user_agent TEXT,
    detail TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_id ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_target_id ON audit_logs(target_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- Add missing columns to users table for quota and security
ALTER TABLE users ADD COLUMN IF NOT EXISTS require_password_change BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS monthly_token_limit BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS monthly_budget_usd DOUBLE PRECISION DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS tokens_invalidated_at TIMESTAMPTZ;

-- Add missing columns to provider_api_keys table
ALTER TABLE provider_api_keys ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 1;
