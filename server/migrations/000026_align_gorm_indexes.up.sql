-- Schema-drift cleanup: GORM AutoMigrate was generating four indexes that no
-- SQL migration had created, breaking `make check-schema`. Add them
-- explicitly so the SQL side matches what GORM would auto-generate.
--
-- Each of these is a low-cost btree on a foreign-key or soft-delete column,
-- exactly what GORM emits for a `gorm:"index"` tag on the corresponding
-- struct field.
--
-- Additionally: `dlp_configs` was created (migration 000006) before the
-- GORM model embedded `BaseModel` (which carries a soft-delete column).
-- GORM AutoMigrate generates both the `deleted_at` column and its index,
-- so the SQL side has to catch up — otherwise the soft-delete the rest of
-- the codebase relies on simply does not work for that table.

ALTER TABLE dlp_configs
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

CREATE INDEX IF NOT EXISTS idx_dlp_configs_deleted_at
    ON dlp_configs(deleted_at);

CREATE INDEX IF NOT EXISTS idx_models_model_kind
    ON models(model_kind);

CREATE INDEX IF NOT EXISTS idx_organizations_owner_id
    ON organizations(owner_id);

CREATE INDEX IF NOT EXISTS idx_projects_org_id
    ON projects(org_id);
