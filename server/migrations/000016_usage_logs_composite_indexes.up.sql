-- Composite indexes for usage_logs hot query paths.
--
-- Every dashboard/aggregation query filters by project_id AND
-- created_at BETWEEN ? AND ?, with optional channel narrowing, then orders by
-- created_at DESC. The pre-existing single-column indexes (idx_usage_logs_project_id,
-- idx_usage_logs_created_at) force either a bitmap-AND or a full sort, which
-- becomes painfully expensive once a tenant accumulates millions of rows.
--
-- Production-deployment note: these are CREATE INDEX IF NOT EXISTS without
-- CONCURRENTLY because golang-migrate wraps each file in a transaction.
-- For large existing tables, operators should instead pre-build the indexes
-- with CREATE INDEX CONCURRENTLY directly against the DB (outside migrate)
-- and then run this file as a no-op (`IF NOT EXISTS` makes that safe).

CREATE INDEX IF NOT EXISTS idx_usage_logs_project_created_at
    ON usage_logs (project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_project_channel_created_at
    ON usage_logs (project_id, channel, created_at DESC);
