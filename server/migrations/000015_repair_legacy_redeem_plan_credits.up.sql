-- Repair legacy redeem codes that were generated as type=plan without a
-- plan_id but with a credit amount. Older redemption logic marked these codes
-- as used without crediting the user's balance.

WITH eligible AS (
    SELECT
        rc.id,
        rc.used_by_id,
        rc.credit_amount
    FROM redeem_codes rc
    WHERE lower(rc.type) = 'plan'
      AND rc.plan_id IS NULL
      AND rc.used_by_id IS NOT NULL
      AND COALESCE(rc.credit_amount, 0) > 0
      AND NOT EXISTS (
          SELECT 1
          FROM transactions t
          WHERE t.reference_id = rc.id::text
            AND t.description = 'Redeem code credit repair'
      )
),
credited AS (
    UPDATE users u
    SET balance = COALESCE(u.balance, 0) + eligible.credit_amount
    FROM eligible
    WHERE u.id = eligible.used_by_id
    RETURNING eligible.id, eligible.used_by_id, eligible.credit_amount, u.balance
)
INSERT INTO transactions (
    id,
    created_at,
    updated_at,
    org_id,
    user_id,
    type,
    amount,
    currency,
    balance,
    description,
    reference_id
)
SELECT
    gen_random_uuid(),
    NOW(),
    NOW(),
    credited.used_by_id,
    credited.used_by_id,
    'recharge',
    credited.credit_amount,
    'USD',
    credited.balance,
    'Redeem code credit repair',
    credited.id::text
FROM credited;

UPDATE redeem_codes
SET type = 'credit'
WHERE lower(type) = 'plan'
  AND plan_id IS NULL
  AND used_by_id IS NOT NULL
  AND COALESCE(credit_amount, 0) > 0;
