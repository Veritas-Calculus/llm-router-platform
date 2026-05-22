-- Webhook delivery exponential backoff.
--
-- Pre-change: ProcessPendingDeliveries ran every 5 seconds and re-attempted
-- every pending row each tick — a single flaky endpoint would consume the
-- HTTP client pool with 12 attempts/min until it crossed the retry-count
-- threshold. Add a next_attempt_at column so the dispatcher can stagger
-- retries via exponential backoff + jitter and honor upstream Retry-After.

ALTER TABLE webhook_deliveries
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ;

-- Backfill: existing pending rows become immediately eligible; everything
-- else is left null and ignored by the new dispatcher filter.
UPDATE webhook_deliveries
SET next_attempt_at = NOW()
WHERE status = 'pending' AND next_attempt_at IS NULL;

-- Partial index supports the new pending-and-due query path.
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_pending_due
    ON webhook_deliveries (next_attempt_at)
    WHERE status = 'pending';
