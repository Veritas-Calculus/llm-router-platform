-- Schema-drift cleanup: align foreign-key constraint NAMES with what GORM
-- AutoMigrate emits, and backfill SQL-side FKs for relationships that
-- only GORM was previously enforcing.
--
-- GORM names FKs `fk_<owner_table>_<child_field>`; the legacy SQL
-- migrations used `<table>_<col>_fkey` (Postgres default) or ad hoc names
-- like `fk_<table>_<short>`. A rename is metadata-only in Postgres — the
-- underlying constraint behaviour, including any ON DELETE/UPDATE action,
-- is preserved.
--
-- Idempotency: this migration may run against a DB where GORM AutoMigrate
-- (dev mode) has already auto-created the destination FK name as a
-- duplicate constraint on the same column. In that case we drop the
-- GORM-added duplicate first (it has no ON DELETE action, so it is the
-- weaker of the pair) and then rename the legacy SQL constraint, which
-- preserves whatever ON DELETE behaviour was declared in earlier
-- migrations. The DO $$ ... $$ blocks make every step safe to re-run.
--
-- ─────────────────────────────────────────────────────────────────────
-- Section A: pure renames — SQL and GORM agree on FK semantics
-- ─────────────────────────────────────────────────────────────────────
-- Both sides have the same FK target with the same ON DELETE action; only
-- the constraint name differs. Drop the GORM duplicate if present, then
-- rename the legacy constraint to the GORM-shape name.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'dlp_configs_project_id_fkey'
                 AND conrelid = 'public.dlp_configs'::regclass) THEN
        ALTER TABLE public.dlp_configs DROP CONSTRAINT IF EXISTS fk_projects_dlp_config;
        ALTER TABLE public.dlp_configs
            RENAME CONSTRAINT dlp_configs_project_id_fkey TO fk_projects_dlp_config;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'models_provider_id_fkey'
                 AND conrelid = 'public.models'::regclass) THEN
        ALTER TABLE public.models DROP CONSTRAINT IF EXISTS fk_providers_models;
        ALTER TABLE public.models
            RENAME CONSTRAINT models_provider_id_fkey TO fk_providers_models;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'provider_api_keys_provider_id_fkey'
                 AND conrelid = 'public.provider_api_keys'::regclass) THEN
        ALTER TABLE public.provider_api_keys DROP CONSTRAINT IF EXISTS fk_provider_api_keys_provider;
        ALTER TABLE public.provider_api_keys
            RENAME CONSTRAINT provider_api_keys_provider_id_fkey TO fk_provider_api_keys_provider;
    END IF;
END $$;

-- ─────────────────────────────────────────────────────────────────────
-- Section B: renames where SQL retains a richer ON DELETE action
-- ─────────────────────────────────────────────────────────────────────
-- GORM AutoMigrate auto-creates FKs without an ON DELETE clause. The SQL
-- migrations explicitly chose CASCADE or RESTRICT for data-integrity
-- reasons (tenancy invariants documented in CLAUDE.md). We drop the GORM
-- duplicate (no ON DELETE) and rename the SQL constraint (with ON DELETE
-- CASCADE or RESTRICT) to the GORM-shape name. The constraint names are
-- added to the check-schema.sh skip list so the pg_dump line difference
-- (cascade vs no-cascade) does not surface as drift — SQL is
-- intentionally stronger.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_api_keys_project'
                 AND conrelid = 'public.api_keys'::regclass) THEN
        ALTER TABLE public.api_keys DROP CONSTRAINT IF EXISTS fk_projects_api_keys;
        ALTER TABLE public.api_keys
            RENAME CONSTRAINT fk_api_keys_project TO fk_projects_api_keys;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'organization_members_org_id_fkey'
                 AND conrelid = 'public.organization_members'::regclass) THEN
        ALTER TABLE public.organization_members DROP CONSTRAINT IF EXISTS fk_organizations_members;
        ALTER TABLE public.organization_members
            RENAME CONSTRAINT organization_members_org_id_fkey TO fk_organizations_members;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'organization_members_user_id_fkey'
                 AND conrelid = 'public.organization_members'::regclass) THEN
        ALTER TABLE public.organization_members DROP CONSTRAINT IF EXISTS fk_users_memberships;
        ALTER TABLE public.organization_members
            RENAME CONSTRAINT organization_members_user_id_fkey TO fk_users_memberships;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'organizations_owner_id_fkey'
                 AND conrelid = 'public.organizations'::regclass) THEN
        ALTER TABLE public.organizations DROP CONSTRAINT IF EXISTS fk_organizations_owner;
        ALTER TABLE public.organizations
            RENAME CONSTRAINT organizations_owner_id_fkey TO fk_organizations_owner;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'projects_org_id_fkey'
                 AND conrelid = 'public.projects'::regclass) THEN
        ALTER TABLE public.projects DROP CONSTRAINT IF EXISTS fk_organizations_projects;
        ALTER TABLE public.projects
            RENAME CONSTRAINT projects_org_id_fkey TO fk_organizations_projects;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_subscriptions_org'
                 AND conrelid = 'public.subscriptions'::regclass) THEN
        ALTER TABLE public.subscriptions DROP CONSTRAINT IF EXISTS fk_subscriptions_organization;
        ALTER TABLE public.subscriptions
            RENAME CONSTRAINT fk_subscriptions_org TO fk_subscriptions_organization;
    END IF;
END $$;

-- ─────────────────────────────────────────────────────────────────────
-- Section C: net-new FK constraints
-- ─────────────────────────────────────────────────────────────────────
-- These FKs are emitted by GORM AutoMigrate but were never added to the
-- SQL side. Each one references a column that existed already, so adding
-- the constraint is metadata-only (Postgres scans the column to validate
-- existing rows; on a typical data set this is fast). No row rewrite.
--
-- Idempotency: DROP CONSTRAINT IF EXISTS before each ADD CONSTRAINT so the
-- migration can be safely re-run after a partial failure (or in
-- environments where GORM AutoMigrate already created the same FK at
-- startup).
--
-- Where existing rows might violate the FK on a real database (e.g. a
-- legacy NULLable column that was filled before the referenced table
-- existed) NOT VALID could be used. We do not need it here because every
-- column below is either NULLable (NULLs satisfy any FK) or has only ever
-- been written via the GORM model that maintains referential integrity.

ALTER TABLE public.provider_api_keys
    DROP CONSTRAINT IF EXISTS fk_provider_api_keys_proxy;
ALTER TABLE public.provider_api_keys
    ADD CONSTRAINT fk_provider_api_keys_proxy
    FOREIGN KEY (proxy_id) REFERENCES public.proxies(id);

ALTER TABLE public.provider_api_keys
    DROP CONSTRAINT IF EXISTS fk_provider_api_keys_proxy_pool;
ALTER TABLE public.provider_api_keys
    ADD CONSTRAINT fk_provider_api_keys_proxy_pool
    FOREIGN KEY (proxy_pool_id) REFERENCES public.proxy_pools(id);

ALTER TABLE public.proxies
    DROP CONSTRAINT IF EXISTS fk_proxy_pools_proxies;
ALTER TABLE public.proxies
    ADD CONSTRAINT fk_proxy_pools_proxies
    FOREIGN KEY (pool_id) REFERENCES public.proxy_pools(id);

ALTER TABLE public.proxies
    DROP CONSTRAINT IF EXISTS fk_proxies_upstream_proxy;
ALTER TABLE public.proxies
    ADD CONSTRAINT fk_proxies_upstream_proxy
    FOREIGN KEY (upstream_proxy_id) REFERENCES public.proxies(id);

ALTER TABLE public.redeem_codes
    DROP CONSTRAINT IF EXISTS fk_redeem_codes_used_by;
ALTER TABLE public.redeem_codes
    ADD CONSTRAINT fk_redeem_codes_used_by
    FOREIGN KEY (used_by_id) REFERENCES public.users(id);

ALTER TABLE public.routing_rules
    DROP CONSTRAINT IF EXISTS fk_routing_rules_fallback_provider;
ALTER TABLE public.routing_rules
    ADD CONSTRAINT fk_routing_rules_fallback_provider
    FOREIGN KEY (fallback_provider_id) REFERENCES public.providers(id);

ALTER TABLE public.routing_rules
    DROP CONSTRAINT IF EXISTS fk_routing_rules_target_provider;
ALTER TABLE public.routing_rules
    ADD CONSTRAINT fk_routing_rules_target_provider
    FOREIGN KEY (target_provider_id) REFERENCES public.providers(id);
