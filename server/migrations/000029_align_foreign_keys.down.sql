-- Reverse of 000029_align_foreign_keys.up.sql.
--
-- Idempotency: each RENAME is wrapped in a DO block that checks the
-- source (GORM-shape) constraint still exists. If the reverse migration
-- is re-applied (or if GORM AutoMigrate has interfered between the up
-- and down), it remains a no-op rather than erroring.

-- C. Drop the FKs added in Section C
ALTER TABLE public.routing_rules     DROP CONSTRAINT IF EXISTS fk_routing_rules_target_provider;
ALTER TABLE public.routing_rules     DROP CONSTRAINT IF EXISTS fk_routing_rules_fallback_provider;
ALTER TABLE public.redeem_codes      DROP CONSTRAINT IF EXISTS fk_redeem_codes_used_by;
ALTER TABLE public.proxies           DROP CONSTRAINT IF EXISTS fk_proxies_upstream_proxy;
ALTER TABLE public.proxies           DROP CONSTRAINT IF EXISTS fk_proxy_pools_proxies;
ALTER TABLE public.provider_api_keys DROP CONSTRAINT IF EXISTS fk_provider_api_keys_proxy_pool;
ALTER TABLE public.provider_api_keys DROP CONSTRAINT IF EXISTS fk_provider_api_keys_proxy;

-- B. Rename back to the legacy names
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_subscriptions_organization'
                 AND conrelid = 'public.subscriptions'::regclass) THEN
        ALTER TABLE public.subscriptions DROP CONSTRAINT IF EXISTS fk_subscriptions_org;
        ALTER TABLE public.subscriptions
            RENAME CONSTRAINT fk_subscriptions_organization TO fk_subscriptions_org;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_organizations_projects'
                 AND conrelid = 'public.projects'::regclass) THEN
        ALTER TABLE public.projects DROP CONSTRAINT IF EXISTS projects_org_id_fkey;
        ALTER TABLE public.projects
            RENAME CONSTRAINT fk_organizations_projects TO projects_org_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_organizations_owner'
                 AND conrelid = 'public.organizations'::regclass) THEN
        ALTER TABLE public.organizations DROP CONSTRAINT IF EXISTS organizations_owner_id_fkey;
        ALTER TABLE public.organizations
            RENAME CONSTRAINT fk_organizations_owner TO organizations_owner_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_users_memberships'
                 AND conrelid = 'public.organization_members'::regclass) THEN
        ALTER TABLE public.organization_members DROP CONSTRAINT IF EXISTS organization_members_user_id_fkey;
        ALTER TABLE public.organization_members
            RENAME CONSTRAINT fk_users_memberships TO organization_members_user_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_organizations_members'
                 AND conrelid = 'public.organization_members'::regclass) THEN
        ALTER TABLE public.organization_members DROP CONSTRAINT IF EXISTS organization_members_org_id_fkey;
        ALTER TABLE public.organization_members
            RENAME CONSTRAINT fk_organizations_members TO organization_members_org_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_projects_api_keys'
                 AND conrelid = 'public.api_keys'::regclass) THEN
        ALTER TABLE public.api_keys DROP CONSTRAINT IF EXISTS fk_api_keys_project;
        ALTER TABLE public.api_keys
            RENAME CONSTRAINT fk_projects_api_keys TO fk_api_keys_project;
    END IF;
END $$;

-- A. Pure-rename revert
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_provider_api_keys_provider'
                 AND conrelid = 'public.provider_api_keys'::regclass) THEN
        ALTER TABLE public.provider_api_keys DROP CONSTRAINT IF EXISTS provider_api_keys_provider_id_fkey;
        ALTER TABLE public.provider_api_keys
            RENAME CONSTRAINT fk_provider_api_keys_provider TO provider_api_keys_provider_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_providers_models'
                 AND conrelid = 'public.models'::regclass) THEN
        ALTER TABLE public.models DROP CONSTRAINT IF EXISTS models_provider_id_fkey;
        ALTER TABLE public.models
            RENAME CONSTRAINT fk_providers_models TO models_provider_id_fkey;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_constraint
               WHERE conname = 'fk_projects_dlp_config'
                 AND conrelid = 'public.dlp_configs'::regclass) THEN
        ALTER TABLE public.dlp_configs DROP CONSTRAINT IF EXISTS dlp_configs_project_id_fkey;
        ALTER TABLE public.dlp_configs
            RENAME CONSTRAINT fk_projects_dlp_config TO dlp_configs_project_id_fkey;
    END IF;
END $$;
