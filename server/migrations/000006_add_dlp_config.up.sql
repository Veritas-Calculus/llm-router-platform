CREATE TABLE IF NOT EXISTS dlp_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    is_enabled BOOLEAN NOT NULL DEFAULT false,
    strategy VARCHAR(32) NOT NULL DEFAULT 'REDACT',
    mask_emails BOOLEAN NOT NULL DEFAULT true,
    mask_phones BOOLEAN NOT NULL DEFAULT true,
    mask_credit_cards BOOLEAN NOT NULL DEFAULT true,
    mask_ssn BOOLEAN NOT NULL DEFAULT true,
    mask_api_keys BOOLEAN NOT NULL DEFAULT true,
    custom_regex JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT idx_dlp_configs_project UNIQUE (project_id)
);

CREATE INDEX IF NOT EXISTS idx_dlp_configs_project_id ON dlp_configs(project_id);
