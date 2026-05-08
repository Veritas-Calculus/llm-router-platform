-- Add proxy pools and account-level proxy bindings for upstream-provider accounts.

CREATE TABLE IF NOT EXISTS proxy_pools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    is_active BOOLEAN DEFAULT true,
    strategy VARCHAR(32) DEFAULT 'weighted'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_proxy_pools_name ON proxy_pools(name);
CREATE INDEX IF NOT EXISTS idx_proxy_pools_deleted_at ON proxy_pools(deleted_at);
CREATE INDEX IF NOT EXISTS idx_proxy_pools_is_active ON proxy_pools(is_active);

ALTER TABLE proxies ADD COLUMN IF NOT EXISTS pool_id UUID;
CREATE INDEX IF NOT EXISTS idx_proxies_pool_id ON proxies(pool_id);

ALTER TABLE provider_api_keys ADD COLUMN IF NOT EXISTS proxy_id UUID;
ALTER TABLE provider_api_keys ADD COLUMN IF NOT EXISTS proxy_pool_id UUID;
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_proxy_id ON provider_api_keys(proxy_id);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_proxy_pool_id ON provider_api_keys(proxy_pool_id);

ALTER TABLE provider_api_keys
    ADD CONSTRAINT provider_api_keys_proxy_binding_check
    CHECK (proxy_id IS NULL OR proxy_pool_id IS NULL) NOT VALID;
