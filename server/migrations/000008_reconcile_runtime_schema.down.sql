DROP TABLE IF EXISTS backup_records;
DROP TABLE IF EXISTS notification_channels;
DROP TABLE IF EXISTS cache_configs;
DROP TABLE IF EXISTS prompt_versions;
DROP TABLE IF EXISTS prompt_templates;
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_endpoints;
DROP TABLE IF EXISTS identity_providers;
DROP TABLE IF EXISTS email_verification_tokens;

DROP INDEX IF EXISTS idx_transactions_org_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription_id;
DROP INDEX IF EXISTS idx_subscriptions_stripe_customer_id;
DROP INDEX IF EXISTS idx_subscriptions_org_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS stripe_subscription_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS stripe_customer_id;
ALTER TABLE subscriptions DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_orders_org_id;
ALTER TABLE orders DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_budgets_project_id;
DROP INDEX IF EXISTS idx_budgets_org_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS enforce_hard_limit;
ALTER TABLE budgets DROP COLUMN IF EXISTS project_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS org_id;

DROP INDEX IF EXISTS idx_usage_logs_channel;
DROP INDEX IF EXISTS idx_usage_logs_project_id;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS mcp_error_count;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS mcp_call_count;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS bytes_processed;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS item_count;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS duration_ms;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS channel;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_conversation_memories_api_key_id;
DROP INDEX IF EXISTS idx_conversation_memories_project_id;
ALTER TABLE conversation_memories DROP COLUMN IF EXISTS api_key_id;
ALTER TABLE conversation_memories DROP COLUMN IF EXISTS project_id;

DROP INDEX IF EXISTS idx_async_tasks_project_id;
ALTER TABLE async_tasks DROP COLUMN IF EXISTS project_id;

ALTER TABLE alert_configs DROP COLUMN IF EXISTS cooldown_minutes;
ALTER TABLE alert_configs DROP COLUMN IF EXISTS budget_threshold;
ALTER TABLE alert_configs DROP COLUMN IF EXISTS latency_threshold_ms;
ALTER TABLE alert_configs DROP COLUMN IF EXISTS error_rate_threshold;

ALTER TABLE proxies DROP COLUMN IF EXISTS encrypted_password;
ALTER TABLE projects DROP COLUMN IF EXISTS white_listed_ips;

ALTER TABLE models DROP COLUMN IF EXISTS price_per_minute;
ALTER TABLE models DROP COLUMN IF EXISTS price_per_image;
ALTER TABLE models DROP COLUMN IF EXISTS price_per_second;
ALTER TABLE providers DROP COLUMN IF EXISTS model_patterns;

ALTER TABLE audit_logs DROP COLUMN IF EXISTS signature;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS previous_hash;

DROP INDEX IF EXISTS idx_api_keys_project_id;
DROP INDEX IF EXISTS idx_api_keys_user_id;
ALTER TABLE api_keys DROP COLUMN IF EXISTS token_limit;
ALTER TABLE api_keys DROP COLUMN IF EXISTS scopes;
ALTER TABLE api_keys DROP COLUMN IF EXISTS user_id;

DROP INDEX IF EXISTS idx_users_o_auth_id;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_backup_codes;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_secret;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS balance;
ALTER TABLE users DROP COLUMN IF EXISTS rate_limit_per_minute;
ALTER TABLE users DROP COLUMN IF EXISTS o_auth_id;
ALTER TABLE users DROP COLUMN IF EXISTS o_auth_provider;
