-- Reverse of 000023. The CASCADE FK is left in place — dropping it is
-- not strictly necessary for rollback safety, but we remove it for symmetry.
ALTER TABLE email_verification_tokens
    DROP CONSTRAINT IF EXISTS email_verification_tokens_user_id_fkey;

ALTER TABLE users
    DROP COLUMN IF EXISTS welcome_credit_granted_at;
