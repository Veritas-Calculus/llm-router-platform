DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'budgets' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE budgets ALTER COLUMN user_id DROP NOT NULL;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'orders' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE orders ALTER COLUMN user_id DROP NOT NULL;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'subscriptions' AND column_name = 'user_id'
    ) THEN
        ALTER TABLE subscriptions ALTER COLUMN user_id DROP NOT NULL;
    END IF;
END $$;

UPDATE subscriptions
SET stripe_customer_id = NULL
WHERE stripe_customer_id = '';

UPDATE subscriptions
SET stripe_subscription_id = NULL
WHERE stripe_subscription_id = '';

DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription_id
    ON subscriptions(stripe_subscription_id)
    WHERE stripe_subscription_id IS NOT NULL;
