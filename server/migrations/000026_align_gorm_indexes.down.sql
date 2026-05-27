-- Reverse of 000026_align_gorm_indexes.up.sql

DROP INDEX IF EXISTS idx_projects_org_id;
DROP INDEX IF EXISTS idx_organizations_owner_id;
DROP INDEX IF EXISTS idx_models_model_kind;
DROP INDEX IF EXISTS idx_dlp_configs_deleted_at;

ALTER TABLE dlp_configs
    DROP COLUMN IF EXISTS deleted_at;
