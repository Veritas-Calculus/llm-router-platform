-- Reconcile SQL migrations with the fields and tables currently used by the
-- release-mode runtime. Without these columns, a fresh database can apply all
-- migrations successfully but fail to seed the default admin user.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS vector;

-- Users / auth
ALTER TABLE users ADD COLUMN IF NOT EXISTS o_auth_provider VARCHAR(32);
ALTER TABLE users ADD COLUMN IF NOT EXISTS o_auth_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS rate_limit_per_minute BIGINT DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS balance NUMERIC DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_enabled BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_secret VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS mfa_backup_codes TEXT;
CREATE INDEX IF NOT EXISTS idx_users_o_auth_id ON users(o_auth_id);

-- API keys
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS user_id UUID;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS scopes TEXT DEFAULT 'all';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS token_limit BIGINT DEFAULT 0;

UPDATE api_keys ak
SET user_id = o.owner_id
FROM projects p
JOIN organizations o ON o.id = p.org_id
WHERE ak.project_id = p.id
  AND ak.user_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM api_keys WHERE user_id IS NULL) THEN
        ALTER TABLE api_keys ALTER COLUMN user_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_project_id ON api_keys(project_id);

-- Audit log integrity chain
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS previous_hash VARCHAR(64);
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS signature VARCHAR(64);

-- Provider/model routing and multimodal pricing
ALTER TABLE providers ADD COLUMN IF NOT EXISTS model_patterns JSONB;
ALTER TABLE models ADD COLUMN IF NOT EXISTS price_per_second NUMERIC DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS price_per_image NUMERIC DEFAULT 0;
ALTER TABLE models ADD COLUMN IF NOT EXISTS price_per_minute NUMERIC DEFAULT 0;

-- Project metadata
ALTER TABLE projects ADD COLUMN IF NOT EXISTS white_listed_ips TEXT;

-- Proxy model compatibility. The field is read-only in Go but still selected by GORM.
ALTER TABLE proxies ADD COLUMN IF NOT EXISTS encrypted_password TEXT;

-- Alerting thresholds
ALTER TABLE alert_configs ADD COLUMN IF NOT EXISTS error_rate_threshold NUMERIC DEFAULT 0;
ALTER TABLE alert_configs ADD COLUMN IF NOT EXISTS latency_threshold_ms BIGINT DEFAULT 0;
ALTER TABLE alert_configs ADD COLUMN IF NOT EXISTS budget_threshold NUMERIC DEFAULT 0;
ALTER TABLE alert_configs ADD COLUMN IF NOT EXISTS cooldown_minutes BIGINT DEFAULT 5;

-- Async task project ownership
ALTER TABLE async_tasks ADD COLUMN IF NOT EXISTS project_id UUID;

UPDATE async_tasks t
SET project_id = (
    SELECT p.id
    FROM projects p
    JOIN organizations o ON o.id = p.org_id
    WHERE o.owner_id = t.user_id
    ORDER BY p.created_at NULLS LAST, p.id
    LIMIT 1
)
WHERE t.project_id IS NULL;

UPDATE async_tasks t
SET project_id = (
    SELECT p.id
    FROM projects p
    JOIN organization_members om ON om.org_id = p.org_id
    WHERE om.user_id = t.user_id
    ORDER BY p.created_at NULLS LAST, p.id
    LIMIT 1
)
WHERE t.project_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM async_tasks WHERE project_id IS NULL) THEN
        ALTER TABLE async_tasks ALTER COLUMN project_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_async_tasks_project_id ON async_tasks(project_id);

-- Conversation memory project/API-key scoping
ALTER TABLE conversation_memories ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE conversation_memories ADD COLUMN IF NOT EXISTS api_key_id UUID;

UPDATE conversation_memories m
SET project_id = (
    SELECT p.id
    FROM projects p
    JOIN organizations o ON o.id = p.org_id
    WHERE o.owner_id = m.user_id
    ORDER BY p.created_at NULLS LAST, p.id
    LIMIT 1
)
WHERE m.project_id IS NULL;

UPDATE conversation_memories m
SET project_id = (
    SELECT p.id
    FROM projects p
    JOIN organization_members om ON om.org_id = p.org_id
    WHERE om.user_id = m.user_id
    ORDER BY p.created_at NULLS LAST, p.id
    LIMIT 1
)
WHERE m.project_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM conversation_memories WHERE project_id IS NULL) THEN
        ALTER TABLE conversation_memories ALTER COLUMN project_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_conversation_memories_project_id ON conversation_memories(project_id);
CREATE INDEX IF NOT EXISTS idx_conversation_memories_api_key_id ON conversation_memories(api_key_id);

-- Usage logs now carry project/channel and non-token metering details.
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS channel TEXT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS duration_ms BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS item_count BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS bytes_processed BIGINT;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS mcp_call_count BIGINT DEFAULT 0;
ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS mcp_error_count BIGINT DEFAULT 0;

UPDATE usage_logs ul
SET project_id = ak.project_id,
    channel = COALESCE(ul.channel, ak.channel, 'default')
FROM api_keys ak
WHERE ul.api_key_id = ak.id
  AND ul.project_id IS NULL;

UPDATE usage_logs ul
SET project_id = (
    SELECT p.id
    FROM projects p
    JOIN organizations o ON o.id = p.org_id
    WHERE o.owner_id = ul.user_id
    ORDER BY p.created_at NULLS LAST, p.id
    LIMIT 1
)
WHERE ul.project_id IS NULL;

UPDATE usage_logs SET channel = 'default' WHERE channel IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM usage_logs WHERE project_id IS NULL) THEN
        ALTER TABLE usage_logs ALTER COLUMN project_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_usage_logs_project_id ON usage_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_channel ON usage_logs(channel);

-- Organization-based billing columns. Old user_id columns are intentionally
-- kept for rollback compatibility and historical data inspection.
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS org_id UUID;
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS project_id UUID;
ALTER TABLE budgets ADD COLUMN IF NOT EXISTS enforce_hard_limit BOOLEAN DEFAULT false;

UPDATE budgets b
SET org_id = o.id
FROM organizations o
WHERE b.user_id = o.owner_id
  AND b.org_id IS NULL;

UPDATE budgets b
SET org_id = om.org_id
FROM organization_members om
WHERE b.user_id = om.user_id
  AND b.org_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM budgets WHERE org_id IS NULL) THEN
        ALTER TABLE budgets ALTER COLUMN org_id SET NOT NULL;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM budgets
        WHERE org_id IS NOT NULL
        GROUP BY org_id
        HAVING COUNT(*) > 1
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_org_id ON budgets(org_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_budgets_project_id ON budgets(project_id);

ALTER TABLE orders ADD COLUMN IF NOT EXISTS org_id UUID;

UPDATE orders o
SET org_id = org.id
FROM organizations org
WHERE o.user_id = org.owner_id
  AND o.org_id IS NULL;

UPDATE orders o
SET org_id = om.org_id
FROM organization_members om
WHERE o.user_id = om.user_id
  AND o.org_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM orders WHERE org_id IS NULL) THEN
        ALTER TABLE orders ALTER COLUMN org_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_orders_org_id ON orders(org_id);

ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS org_id UUID;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS stripe_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS stripe_subscription_id TEXT;

UPDATE subscriptions s
SET org_id = org.id
FROM organizations org
WHERE s.user_id = org.owner_id
  AND s.org_id IS NULL;

UPDATE subscriptions s
SET org_id = om.org_id
FROM organization_members om
WHERE s.user_id = om.user_id
  AND s.org_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM subscriptions WHERE org_id IS NULL) THEN
        ALTER TABLE subscriptions ALTER COLUMN org_id SET NOT NULL;
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM subscriptions
        WHERE org_id IS NOT NULL
        GROUP BY org_id
        HAVING COUNT(*) > 1
    ) THEN
        CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_org_id ON subscriptions(org_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_customer_id ON subscriptions(stripe_customer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription_id
    ON subscriptions(stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS org_id UUID;

UPDATE transactions t
SET org_id = org.id
FROM organizations org
WHERE t.user_id = org.owner_id
  AND t.org_id IS NULL;

UPDATE transactions t
SET org_id = om.org_id
FROM organization_members om
WHERE t.user_id = om.user_id
  AND t.org_id IS NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM transactions WHERE org_id IS NULL) THEN
        ALTER TABLE transactions ALTER COLUMN org_id SET NOT NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_transactions_org_id ON transactions(org_id);

-- Tables introduced in models but missing from release SQL migrations.
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    user_id UUID NOT NULL,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_created_at ON email_verification_tokens(created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_verification_tokens_token_hash ON email_verification_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id ON email_verification_tokens(user_id);

CREATE TABLE IF NOT EXISTS identity_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    org_id UUID NOT NULL,
    type VARCHAR(32) NOT NULL,
    name VARCHAR(128) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    domains TEXT,
    o_id_c_client_id VARCHAR(255),
    o_id_c_client_secret VARCHAR(255),
    o_id_c_issuer_url TEXT,
    saml_entity_id TEXT,
    samlsso_url TEXT,
    saml_id_p_cert TEXT,
    enable_jit BOOLEAN DEFAULT true,
    default_role VARCHAR(64) DEFAULT 'MEMBER',
    group_role_mapping TEXT
);
CREATE INDEX IF NOT EXISTS idx_identity_providers_deleted_at ON identity_providers(deleted_at);
CREATE INDEX IF NOT EXISTS idx_identity_providers_org_id ON identity_providers(org_id);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_identity_providers_organization') THEN
        ALTER TABLE identity_providers
            ADD CONSTRAINT fk_identity_providers_organization
            FOREIGN KEY (org_id) REFERENCES organizations(id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    project_id UUID NOT NULL,
    url TEXT NOT NULL,
    secret VARCHAR(255) NOT NULL,
    events JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    description TEXT
);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_deleted_at ON webhook_endpoints(deleted_at);
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_project_id ON webhook_endpoints(project_id);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_webhook_endpoints_project') THEN
        ALTER TABLE webhook_endpoints
            ADD CONSTRAINT fk_webhook_endpoints_project
            FOREIGN KEY (project_id) REFERENCES projects(id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    endpoint_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    status_code BIGINT NOT NULL DEFAULT 0,
    response_body TEXT,
    error_message TEXT,
    retry_count BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_deleted_at ON webhook_deliveries(deleted_at);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint_id ON webhook_deliveries(endpoint_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_webhook_deliveries_endpoint') THEN
        ALTER TABLE webhook_deliveries
            ADD CONSTRAINT fk_webhook_deliveries_endpoint
            FOREIGN KEY (endpoint_id) REFERENCES webhook_endpoints(id) ON DELETE CASCADE;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS prompt_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    name TEXT NOT NULL,
    description TEXT,
    project_id UUID,
    is_active BOOLEAN DEFAULT true,
    active_version_id UUID
);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_deleted_at ON prompt_templates(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_templates_name ON prompt_templates(name);
CREATE INDEX IF NOT EXISTS idx_prompt_templates_project_id ON prompt_templates(project_id);

CREATE TABLE IF NOT EXISTS prompt_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    template_id UUID NOT NULL,
    version BIGINT NOT NULL,
    content TEXT NOT NULL,
    model TEXT,
    parameters JSONB,
    change_log TEXT
);
CREATE INDEX IF NOT EXISTS idx_prompt_versions_deleted_at ON prompt_versions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_prompt_versions_template_id ON prompt_versions(template_id);
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_prompt_templates_versions') THEN
        ALTER TABLE prompt_versions
            ADD CONSTRAINT fk_prompt_templates_versions
            FOREIGN KEY (template_id) REFERENCES prompt_templates(id);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS cache_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    is_enabled BOOLEAN DEFAULT true,
    similarity_threshold NUMERIC DEFAULT 0.05,
    default_ttl_minutes BIGINT DEFAULT 1440,
    embedding_model VARCHAR(128) DEFAULT 'text-embedding-3-small',
    max_cache_size BIGINT DEFAULT 10000
);
CREATE INDEX IF NOT EXISTS idx_cache_configs_deleted_at ON cache_configs(deleted_at);

CREATE TABLE IF NOT EXISTS notification_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    config TEXT,
    created_by UUID
);
CREATE INDEX IF NOT EXISTS idx_notification_channels_deleted_at ON notification_channels(deleted_at);

CREATE TABLE IF NOT EXISTS backup_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(32) NOT NULL DEFAULT 'manual',
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    size_bytes BIGINT DEFAULT 0,
    duration_ms BIGINT DEFAULT 0,
    destination TEXT,
    error_msg TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ
);
