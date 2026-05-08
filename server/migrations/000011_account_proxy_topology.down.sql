ALTER TABLE provider_api_keys DROP CONSTRAINT IF EXISTS provider_api_keys_proxy_binding_check;
DROP INDEX IF EXISTS idx_provider_api_keys_proxy_pool_id;
DROP INDEX IF EXISTS idx_provider_api_keys_proxy_id;
ALTER TABLE provider_api_keys DROP COLUMN IF EXISTS proxy_pool_id;
ALTER TABLE provider_api_keys DROP COLUMN IF EXISTS proxy_id;

DROP INDEX IF EXISTS idx_proxies_pool_id;
ALTER TABLE proxies DROP COLUMN IF EXISTS pool_id;

DROP INDEX IF EXISTS idx_proxy_pools_is_active;
DROP INDEX IF EXISTS idx_proxy_pools_deleted_at;
DROP INDEX IF EXISTS idx_proxy_pools_name;
DROP TABLE IF EXISTS proxy_pools CASCADE;
