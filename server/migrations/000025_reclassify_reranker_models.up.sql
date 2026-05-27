-- Post-audit P0-1: repair `model_kind` for reranker rows that migration
-- 000024 mis-backfilled as embeddings.
--
-- Migration 000024 used this CASE WHEN ladder to backfill model_kind:
--
--   WHEN LOWER(name) ~ '(embedding|embed|bge-|nomic-embed)' THEN 'embedding'
--   ...
--   WHEN LOWER(name) ~ '(rerank|reranker)' THEN 'rerank'
--
-- Because the embedding branch fires first, any row whose name matches
-- BOTH the embedding pattern (e.g. via the `bge-` prefix) AND the rerank
-- pattern was classified as embedding. The most common offender is
-- `bge-reranker-v2-m3`, which is in fact a reranker — see the Go-side
-- classifier in server/internal/service/router/model_classification.go,
-- where rerank deliberately wins over embedding.
--
-- The next time SyncProviderModels runs the row would self-heal because
-- the Go classifier picks rerank first, but until then admin UIs and any
-- rerank-aware routing are using the wrong bucket. This migration narrows
-- the fix to rows where the name clearly identifies a reranker.

UPDATE models
SET model_kind = 'rerank'
WHERE model_kind = 'embedding'
  AND LOWER(name) ~ '(rerank|reranker)';
