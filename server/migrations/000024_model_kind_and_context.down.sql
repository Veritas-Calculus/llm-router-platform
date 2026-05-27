-- Reverse of 000024. We drop the new columns and the helper index but do
-- NOT attempt to re-activate rows that were deactivated by the NSFW
-- backfill — there is no way to know which rows were inactive before this
-- migration ran. An operator can re-enable individual rows from the admin UI.

DROP INDEX IF EXISTS idx_models_provider_kind;

ALTER TABLE models
    DROP CONSTRAINT IF EXISTS models_model_kind_check;

ALTER TABLE models
    DROP COLUMN IF EXISTS catalog_warnings,
    DROP COLUMN IF EXISTS max_output_tokens,
    DROP COLUMN IF EXISTS context_window,
    DROP COLUMN IF EXISTS model_kind;
