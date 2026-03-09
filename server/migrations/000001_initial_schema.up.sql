-- 000001_initial_schema.up.sql
-- Initial schema for LLM Router Platform
-- Matches the GORM AutoMigrate output as of v1.0

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50) DEFAULT 'user',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users(id),
    key_hash VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    name VARCHAR(255),
    is_active BOOLEAN DEFAULT true,
    rate_limit INTEGER DEFAULT 1000,
    daily_limit INTEGER DEFAULT 10000,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_deleted_at ON api_keys(deleted_at);

-- Providers table
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    is_active BOOLEAN DEFAULT true,
    priority INTEGER DEFAULT 0,
    weight DOUBLE PRECISION DEFAULT 1.0,
    max_retries INTEGER DEFAULT 3,
    timeout INTEGER DEFAULT 30,
    use_proxy BOOLEAN DEFAULT false,
    default_proxy_id UUID,
    requires_api_key BOOLEAN DEFAULT true
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_name ON providers(name);
CREATE INDEX IF NOT EXISTS idx_providers_deleted_at ON providers(deleted_at);

-- Models table
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    provider_id UUID NOT NULL REFERENCES providers(id),
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    input_price_per1_k DOUBLE PRECISION DEFAULT 0,
    output_price_per1_k DOUBLE PRECISION DEFAULT 0,
    max_tokens INTEGER DEFAULT 4096,
    is_active BOOLEAN DEFAULT true
);
CREATE INDEX IF NOT EXISTS idx_models_provider_id ON models(provider_id);
CREATE INDEX IF NOT EXISTS idx_models_deleted_at ON models(deleted_at);

-- Provider API Keys table
CREATE TABLE IF NOT EXISTS provider_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    provider_id UUID NOT NULL REFERENCES providers(id),
    alias VARCHAR(255),
    encrypted_api_key TEXT NOT NULL,
    key_prefix VARCHAR(20),
    is_active BOOLEAN DEFAULT true,
    weight DOUBLE PRECISION DEFAULT 1.0,
    rate_limit INTEGER DEFAULT 0,
    usage_count BIGINT DEFAULT 0,
    last_used_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_provider_id ON provider_api_keys(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_deleted_at ON provider_api_keys(deleted_at);

-- Proxies table
CREATE TABLE IF NOT EXISTS proxies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    url VARCHAR(500) NOT NULL,
    type VARCHAR(20) DEFAULT 'http',
    username VARCHAR(255),
    password VARCHAR(500),
    region VARCHAR(100),
    upstream_proxy_id UUID,
    is_active BOOLEAN DEFAULT true,
    weight DOUBLE PRECISION DEFAULT 1.0,
    success_count BIGINT DEFAULT 0,
    failure_count BIGINT DEFAULT 0,
    avg_latency DOUBLE PRECISION DEFAULT 0,
    last_checked TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_proxies_upstream_proxy_id ON proxies(upstream_proxy_id);
CREATE INDEX IF NOT EXISTS idx_proxies_deleted_at ON proxies(deleted_at);

-- Usage Logs table
CREATE TABLE IF NOT EXISTS usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL,
    api_key_id UUID NOT NULL,
    provider_id UUID,
    model_id UUID,
    model_name VARCHAR(255),
    proxy_id UUID,
    request_tokens INTEGER,
    response_tokens INTEGER,
    total_tokens INTEGER,
    cost DOUBLE PRECISION,
    latency BIGINT,
    status_code INTEGER,
    error_message TEXT
);
CREATE INDEX IF NOT EXISTS idx_usage_logs_user_id ON usage_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_api_key_id ON usage_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_provider_id ON usage_logs(provider_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_model_id ON usage_logs(model_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_model_name ON usage_logs(model_name);
CREATE INDEX IF NOT EXISTS idx_usage_logs_proxy_id ON usage_logs(proxy_id);
CREATE INDEX IF NOT EXISTS idx_usage_logs_deleted_at ON usage_logs(deleted_at);
CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at ON usage_logs(created_at);

-- Health History table
CREATE TABLE IF NOT EXISTS health_histories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,
    is_healthy BOOLEAN,
    response_time BIGINT,
    error_message TEXT,
    checked_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_health_histories_target_type ON health_histories(target_type);
CREATE INDEX IF NOT EXISTS idx_health_histories_target_id ON health_histories(target_id);
CREATE INDEX IF NOT EXISTS idx_health_histories_checked_at ON health_histories(checked_at);
CREATE INDEX IF NOT EXISTS idx_health_histories_deleted_at ON health_histories(deleted_at);

-- Alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,
    alert_type VARCHAR(50) NOT NULL,
    message TEXT,
    status VARCHAR(20) DEFAULT 'active',
    acknowledged_at TIMESTAMPTZ,
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_alerts_target_type ON alerts(target_type);
CREATE INDEX IF NOT EXISTS idx_alerts_target_id ON alerts(target_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_deleted_at ON alerts(deleted_at);

-- Alert Configs table
CREATE TABLE IF NOT EXISTS alert_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    failure_threshold INTEGER DEFAULT 3,
    webhook_url VARCHAR(500),
    email VARCHAR(255)
);
CREATE INDEX IF NOT EXISTS idx_alert_configs_target_type ON alert_configs(target_type);
CREATE INDEX IF NOT EXISTS idx_alert_configs_target_id ON alert_configs(target_id);
CREATE INDEX IF NOT EXISTS idx_alert_configs_deleted_at ON alert_configs(deleted_at);

-- Conversation Memory table
CREATE TABLE IF NOT EXISTS conversation_memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL,
    conversation_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT,
    token_count INTEGER,
    sequence INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_conversation_memories_user_id ON conversation_memories(user_id);
CREATE INDEX IF NOT EXISTS idx_conversation_memories_conversation_id ON conversation_memories(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_memories_deleted_at ON conversation_memories(deleted_at);
