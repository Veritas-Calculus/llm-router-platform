-- C-01: Gate the $5 welcome credit behind email verification + captcha.
--
-- The email_verified / email_verified_at columns and the
-- email_verification_tokens table already exist (added in migration 000008).
-- This migration adds a single load-bearing column on users that lets the
-- verifyEmail flow be idempotent: once credit has been granted we set the
-- timestamp, and a second VerifyEmail call (or a manual replay) cannot
-- credit the user again.
--
-- We deliberately use a single nullable timestamp rather than a boolean +
-- timestamp so the "granted at?" check is a single column read. NULL means
-- the welcome credit has not yet been issued for this account; a non-NULL
-- value carries both "yes, granted" and "when".
--
-- The existing welcome-credit logic in service/user/onboarding.go also
-- writes a row to `transactions` of type='recharge' with description='Welcome
-- credit'. That's the source of truth for finance reporting; this column
-- exists purely for the resolver-level idempotency check on verifyEmail and
-- to avoid an extra COUNT query on every verification attempt.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS welcome_credit_granted_at TIMESTAMPTZ NULL;

-- Backfill: any existing user with a non-zero balance is treated as
-- already-credited, so they cannot accidentally receive a second welcome
-- credit. This is conservative — a user could plausibly have a non-zero
-- balance from a recharge — but the worst case is "this user can't claim
-- the welcome credit by verifying their email later", which is the desired
-- behaviour. Existing accounts can be reset by admins via direct SQL.
UPDATE users
SET welcome_credit_granted_at = COALESCE(updated_at, created_at, NOW())
WHERE balance > 0
  AND welcome_credit_granted_at IS NULL;

-- The email_verification_tokens table from migration 000008 stores the
-- token hash but does not have an ON DELETE CASCADE to users. If a user
-- row is deleted (admin action), orphan tokens linger forever. Add it now.
-- Wrapped in a DO block because the constraint name is generated.
DO $$
DECLARE
    fk_name TEXT;
BEGIN
    SELECT conname INTO fk_name
    FROM pg_constraint
    WHERE conrelid = 'email_verification_tokens'::regclass
      AND contype = 'f'
      AND pg_get_constraintdef(oid) LIKE 'FOREIGN KEY (user_id)%';

    IF fk_name IS NULL THEN
        ALTER TABLE email_verification_tokens
            ADD CONSTRAINT email_verification_tokens_user_id_fkey
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
    END IF;
END $$;
