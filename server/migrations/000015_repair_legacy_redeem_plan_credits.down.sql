UPDATE users u
SET balance = COALESCE(u.balance, 0) - repair.amount
FROM transactions repair
WHERE repair.description = 'Redeem code credit repair'
  AND repair.user_id = u.id;

UPDATE redeem_codes rc
SET type = 'plan'
FROM transactions repair
WHERE repair.description = 'Redeem code credit repair'
  AND repair.reference_id = rc.id::text;

DELETE FROM transactions
WHERE description = 'Redeem code credit repair';
