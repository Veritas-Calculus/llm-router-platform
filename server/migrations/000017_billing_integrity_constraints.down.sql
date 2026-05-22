DROP INDEX IF EXISTS idx_transactions_idempotency_key;
ALTER TABLE transactions DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE users DROP CONSTRAINT IF EXISTS chk_users_balance_nonnegative;
