-- Billing integrity constraints:
--   1. users.balance must never be negative.
--   2. transactions can carry an idempotency_key that is unique when present,
--      so external webhook retries (Stripe SETNX miss, WeChat double-deliver,
--      Alipay async retry) cannot apply the same credit twice.
--
-- The CHECK is added in two steps so existing rows that may have been
-- overdrafted by the pre-fix DeductBalance code path don't abort the ALTER.
-- We floor any negative balance to 0 first and record a repair transaction
-- for the audit trail.

DO $$
DECLARE
    repaired_count int;
BEGIN
    SELECT COUNT(*) INTO repaired_count FROM users WHERE balance < 0;
    IF repaired_count > 0 THEN
        INSERT INTO transactions (id, user_id, org_id, type, amount, balance, currency, description, created_at, updated_at)
        SELECT
            gen_random_uuid(),
            u.id,
            u.id,                       -- personal-workspace convention
            'repair',
            -u.balance,                  -- credit the overdraft back to zero
            0,
            'USD',
            'Floor negative balance during 000017 integrity migration',
            NOW(),
            NOW()
        FROM users u WHERE u.balance < 0;

        UPDATE users SET balance = 0 WHERE balance < 0;

        RAISE NOTICE 'Repaired % users with negative balance', repaired_count;
    END IF;
END $$;

ALTER TABLE users
    ADD CONSTRAINT chk_users_balance_nonnegative CHECK (balance >= 0);

ALTER TABLE transactions
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_idempotency_key
    ON transactions(idempotency_key) WHERE idempotency_key IS NOT NULL;
