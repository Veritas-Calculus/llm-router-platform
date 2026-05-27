-- Reverse of 000025. Re-classify any reranker-named rows back to embedding
-- so the DB state matches what 000024_up produced. Operators who manually
-- corrected a row to a different kind in the meantime keep that override.

UPDATE models
SET model_kind = 'embedding'
WHERE model_kind = 'rerank'
  AND LOWER(name) ~ '(rerank|reranker)';
