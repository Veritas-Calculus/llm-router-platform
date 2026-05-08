UPDATE providers
SET created_at = NOW()
WHERE created_at IS NULL;

UPDATE providers
SET updated_at = COALESCE(created_at, NOW())
WHERE updated_at IS NULL;

ALTER TABLE providers ALTER COLUMN created_at SET DEFAULT NOW();
ALTER TABLE providers ALTER COLUMN updated_at SET DEFAULT NOW();
ALTER TABLE providers ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE providers ALTER COLUMN updated_at SET NOT NULL;

-- Repair local providers created by older UI/server combinations before
-- requires_api_key=false was reliably persisted.
UPDATE providers p
SET requires_api_key = false,
    is_active = true,
    updated_at = NOW()
WHERE p.requires_api_key = true
  AND NOT EXISTS (
      SELECT 1
      FROM provider_api_keys k
      WHERE k.provider_id = p.id
  )
  AND (
      lower(p.name) IN ('ollama', 'lmstudio', 'vllm')
      OR lower(p.name) LIKE 'ollama-%'
      OR lower(p.name) LIKE 'ollama_%'
      OR lower(p.name) LIKE 'ollama.%'
      OR lower(p.name) LIKE 'lmstudio-%'
      OR lower(p.name) LIKE 'lmstudio_%'
      OR lower(p.name) LIKE 'lmstudio.%'
      OR lower(p.name) LIKE 'vllm-%'
      OR lower(p.name) LIKE 'vllm_%'
      OR lower(p.name) LIKE 'vllm.%'
  );
