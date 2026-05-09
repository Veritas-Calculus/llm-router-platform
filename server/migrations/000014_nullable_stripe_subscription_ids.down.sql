DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription_id;
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_stripe_subscription_id
    ON subscriptions(stripe_subscription_id);
