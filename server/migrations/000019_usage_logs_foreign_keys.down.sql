ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_proxy;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_model;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_provider;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_api_key;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_project;
ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_user;
