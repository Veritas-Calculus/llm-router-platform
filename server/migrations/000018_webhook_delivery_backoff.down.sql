DROP INDEX IF EXISTS idx_webhook_deliveries_pending_due;
ALTER TABLE webhook_deliveries DROP COLUMN IF EXISTS next_attempt_at;
