-- M-02 / M-07 / M-08 / L-05: tighten the provider catalogue's model rows.
--
-- The audit highlighted four overlapping data-quality issues:
--   * Playground STT/TTS dropdowns are populated from /v1/models without any
--     filtering, so users see embedding / whisper / chat models in every
--     dropdown.
--   * Admin Models table reports max_tokens=4096 for every row even though
--     /v1/models returns max_context_length=262144. The single `max_tokens`
--     column was being used as both the context window AND the per-request
--     output cap, which is semantically wrong.
--   * Admin Models table mixes chat, embedding, and audio models in one list
--     with pricing columns that only make sense for chat.
--   * Auto-sync inserts dev-only models like
--     `qwen3.5-35b-a3b-uncensored-hauhaucs-aggressive` straight into the
--     user-facing catalogue.
--
-- This migration introduces three columns on `models` and backfills them
-- from the existing `name` column. The backfill is the L-05 cleanup: any
-- existing row whose name matches the NSFW / dev-only pattern is forced to
-- `is_active = false`. We deliberately DO NOT delete rows — operators may
-- have manually enabled or priced some of them, and silently dropping rows
-- would break referential integrity in any audit / usage tables.
--
-- The original `max_tokens` column is retained for one release as a
-- backwards-compat shim. The application code still reads/writes it for the
-- moment so a rollback (000024_down) leaves the table in a usable state.

-- 1) New columns.
ALTER TABLE models
    ADD COLUMN IF NOT EXISTS model_kind        TEXT    NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS context_window    INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS max_output_tokens INTEGER NULL,
    ADD COLUMN IF NOT EXISTS catalog_warnings  TEXT    NOT NULL DEFAULT '';

-- model_kind is a closed enum. Use a CHECK constraint rather than a
-- pg enum so future kinds (e.g. 'rerank') can be added by code without a
-- schema migration. Wrapped in DO so re-running the migration is safe.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'models_model_kind_check'
    ) THEN
        ALTER TABLE models
            ADD CONSTRAINT models_model_kind_check
            CHECK (model_kind IN ('chat','embedding','image','stt','tts','rerank','unknown'));
    END IF;
END $$;

-- 2) Backfill model_kind from the existing `name`. The ordering matters:
--    STT/TTS keywords are matched before "embedding" so a hypothetical
--    "whisper-embedding-eval" still classifies as stt. The case-insensitive
--    LIKE patterns mirror the Go-side classifier in
--    server/internal/service/router/model_classification.go.
UPDATE models
SET model_kind = CASE
    WHEN LOWER(name) ~ '(whisper|^stt-|/stt-|-stt-|speech-to-text|transcribe)' THEN 'stt'
    WHEN LOWER(name) ~ '(^tts-|/tts-|-tts-|text-to-speech|kokoro|elevenlabs|cosyvoice|bark|parler)' THEN 'tts'
    WHEN LOWER(name) ~ '(embedding|embed|bge-|nomic-embed)' THEN 'embedding'
    WHEN LOWER(name) ~ '(dall-e|stable-diffusion|^sd-|/sd-|flux-?[0-9]|midjourney)' THEN 'image'
    WHEN LOWER(name) ~ '(rerank|reranker)' THEN 'rerank'
    ELSE 'chat'
END
WHERE model_kind = 'unknown';

-- 3) Backfill context_window from the legacy max_tokens column. We are
--    being honest: until the next sync round the operator-supplied
--    max_tokens is the closest thing we have to a context window. Real
--    values from upstream /v1/models will overwrite this on the next
--    SyncProviderModels call.
UPDATE models
SET context_window = COALESCE(NULLIF(max_tokens, 0), 4096)
WHERE context_window = 0;

-- 4) L-05 cleanup: deactivate any existing row whose name matches the
--    NSFW / dev-only pattern. Keep the row (it may have valid pricing,
--    history, or manual operator overrides) but force is_active=false
--    so it stops appearing in /v1/models. Also stamp a warning string
--    so the admin UI can surface why.
UPDATE models
SET is_active        = false,
    catalog_warnings = TRIM(BOTH ' ' FROM CONCAT_WS(' ', NULLIF(catalog_warnings,''), 'nsfw-or-dev-name'))
WHERE LOWER(name) ~ '(uncensored|abliterated|jailbreak|nsfw|aggressive)';

-- 5) Helpful index for the admin UI's grouped queries.
CREATE INDEX IF NOT EXISTS idx_models_provider_kind
    ON models(provider_id, model_kind)
    WHERE deleted_at IS NULL;
