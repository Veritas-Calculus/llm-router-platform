-- usage_logs foreign keys (DB-C4 in the audit roadmap).
--
-- The table holds the source-of-truth for billing and audit, so referential
-- integrity matters. Add FKs with strategies that match the operational
-- contract:
--
--   user_id      ON DELETE RESTRICT  — billing rows MUST stay tied to a user;
--                                       prevent accidental hard-delete.
--   project_id   ON DELETE RESTRICT  — same.
--   api_key_id   ON DELETE RESTRICT  — APIKeyRepository.Delete previously
--                                       used Unscoped().Delete which would
--                                       have orphaned this column. Soft-delete
--                                       (deleted_at) keeps the row addressable.
--   provider_id  ON DELETE SET NULL  — provider can legitimately be removed;
--   model_id     ON DELETE SET NULL    historical usage rows stay, FK goes null.
--   proxy_id     ON DELETE SET NULL
--
-- NOT VALID + VALIDATE CONSTRAINT splits the long lock for tables with existing
-- data so the ALTER TABLE doesn't take a multi-second lock on hot deployments.
-- Operators with very large tables should run VALIDATE during a low-traffic
-- window after the up-migration completes.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_user') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_user
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT NOT VALID;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_project') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_project
            FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE RESTRICT NOT VALID;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_api_key') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_api_key
            FOREIGN KEY (api_key_id) REFERENCES api_keys(id) ON DELETE RESTRICT NOT VALID;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_provider') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_provider
            FOREIGN KEY (provider_id) REFERENCES providers(id) ON DELETE SET NULL NOT VALID;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_model') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_model
            FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE SET NULL NOT VALID;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_usage_logs_proxy') THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT fk_usage_logs_proxy
            FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE SET NULL NOT VALID;
    END IF;
END $$;

-- Validate immediately on small/empty tables; production operators with large
-- usage_logs should rerun VALIDATE in a maintenance window if it takes too long.
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_user;
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_project;
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_api_key;
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_provider;
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_model;
ALTER TABLE usage_logs VALIDATE CONSTRAINT fk_usage_logs_proxy;
