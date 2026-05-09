ALTER TABLE api_keys
  ADD COLUMN IF NOT EXISTS allowed_models JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS allowed_providers JSONB NOT NULL DEFAULT '[]'::jsonb;
