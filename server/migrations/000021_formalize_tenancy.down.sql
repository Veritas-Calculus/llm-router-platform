-- Reverse only the FKs; we don't delete the personal organizations created
-- by the up migration because their org_id is also the user's id, and
-- removing them would orphan billing rows.
ALTER TABLE budgets       DROP CONSTRAINT IF EXISTS fk_budgets_org;
ALTER TABLE subscriptions DROP CONSTRAINT IF EXISTS fk_subscriptions_org;
ALTER TABLE orders        DROP CONSTRAINT IF EXISTS fk_orders_org;
ALTER TABLE transactions  DROP CONSTRAINT IF EXISTS fk_transactions_org;
