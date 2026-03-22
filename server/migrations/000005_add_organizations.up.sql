-- 000005_add_organizations.up.sql

-- 1. Create tables for Organizations and Projects hierarchy
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    billing_limit DECIMAL(20, 4) DEFAULT 0.0000,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_organizations_deleted_at ON organizations(deleted_at);

CREATE TABLE IF NOT EXISTS organization_members (
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(64) NOT NULL DEFAULT 'MEMBER',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (org_id, user_id)
);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    quota_limit DECIMAL(20, 4) DEFAULT 0.0000,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);
CREATE INDEX IF NOT EXISTS idx_projects_deleted_at ON projects(deleted_at);

-- 2. Alter api_keys table
ALTER TABLE api_keys ADD COLUMN project_id UUID;
ALTER TABLE api_keys ADD COLUMN channel VARCHAR(128) DEFAULT 'default';

-- 3. Soft data migration: create Personal Workspace for each user who has API keys (or all users)
-- First, we safely ensure every user gets a personal organization and project
DO $$
DECLARE
    u_record RECORD;
    new_org_id UUID;
    new_proj_id UUID;
BEGIN
    FOR u_record IN SELECT id FROM users
    LOOP
        -- Create Personal Workspace Org
        new_org_id := gen_random_uuid();
        INSERT INTO organizations (id, name, owner_id) 
        VALUES (new_org_id, 'Personal Workspace', u_record.id);
        
        -- Add user to org members as OWNER
        INSERT INTO organization_members (org_id, user_id, role)
        VALUES (new_org_id, u_record.id, 'OWNER');

        -- Create Personal Project
        new_proj_id := gen_random_uuid();
        INSERT INTO projects (id, org_id, name, description)
        VALUES (new_proj_id, new_org_id, 'Personal Project', 'Default project for personal usage');

        -- Associate all existing API keys for this user to this new project
        UPDATE api_keys 
        SET project_id = new_proj_id 
        WHERE user_id = u_record.id AND project_id IS NULL;
    END LOOP;
END $$;

-- 4. Delete orphaned API keys that don't belong to any user (precautionary)
DELETE FROM api_keys WHERE project_id IS NULL;

-- 5. Enforce project_id as NOT NULL, and set up foreign key
ALTER TABLE api_keys ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE api_keys ADD CONSTRAINT fk_api_keys_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE;

-- 6. Drop user_id from api_keys since it's now governed by project->org->member hierarchy
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_user_id_fkey;
ALTER TABLE api_keys DROP COLUMN user_id;
