-- Revert constraint rename
ALTER TABLE dlp_configs RENAME CONSTRAINT uni_dlp_configs_project_id TO idx_dlp_configs_project;
