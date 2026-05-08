-- Providers are managed through GraphQL and expose createdAt as non-null.
-- Older runtime-created rows may have NULL timestamps because the live schema
-- did not always carry defaults. Backfill them and make future inserts safe.

UPDATE providers
SET created_at = NOW()
WHERE created_at IS NULL;

UPDATE providers
SET updated_at = COALESCE(created_at, NOW())
WHERE updated_at IS NULL;

ALTER TABLE providers
    ALTER COLUMN created_at SET DEFAULT NOW(),
    ALTER COLUMN updated_at SET DEFAULT NOW(),
    ALTER COLUMN created_at SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL;
