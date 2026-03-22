-- 000005_add_organizations.down.sql

ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS fk_api_keys_project;
ALTER TABLE api_keys DROP COLUMN IF EXISTS channel;
ALTER TABLE api_keys ADD COLUMN user_id UUID;

-- Since we cannot deterministically restore the exact api_keys to user_id mapping without a backup,
-- we will attempt to restore it based on the project -> org -> owner hierarchy
UPDATE api_keys ak
SET user_id = (
    SELECT o.owner_id 
    FROM projects p
    JOIN organizations o ON p.org_id = o.id
    WHERE p.id = ak.project_id
);

-- Delete orphaned keys
DELETE FROM api_keys WHERE user_id IS NULL;

-- Enforce original NOT NULL constraint and foreign key
ALTER TABLE api_keys ALTER COLUMN user_id SET NOT NULL;
ALTER TABLE api_keys ADD CONSTRAINT api_keys_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Drop project_id column
ALTER TABLE api_keys DROP COLUMN IF EXISTS project_id;

-- Drop tables
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS organization_members CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;
