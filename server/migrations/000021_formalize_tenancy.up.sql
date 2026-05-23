-- Formalize the tenancy model so transactions/orders/subscriptions/budgets
-- always point at a real organizations row (DB-C3 in the audit roadmap).
--
-- Pre-migration state: billing tables stored the USER's id in the org_id
-- column whenever the user had no real organization. There was no FK from
-- billing.org_id → organizations.id, so the convention was enforced only by
-- handler code and broke down anywhere it didn't (and made future per-org
-- analytics quietly miscount).
--
-- Strategy: every user gets a personal organization. We use the user's UUID
-- as the org UUID so existing billing rows (which already store user_id in
-- org_id) become valid without any data migration. The Org.OwnerID points
-- back at the user. The handler-level "use user_id as org_id" convention
-- now matches an actual row in organizations.
--
-- After this migration, billing FKs can land safely (a follow-up commit).

DO $$
DECLARE
    backfilled int := 0;
BEGIN
    -- Create a personal org for every user that doesn't already have one
    -- with their UUID. We avoid touching users that have already been
    -- assigned an organization manually (those rows will have a different
    -- (id != user_id) entry — leave them; their billing already points at
    -- the real org id).
    INSERT INTO organizations (id, owner_id, name, billing_limit, created_at, updated_at, deleted_at)
    SELECT
        u.id,
        u.id,
        COALESCE(NULLIF(u.name, ''), split_part(u.email, '@', 1)) || '''s workspace',
        0,
        COALESCE(u.created_at, NOW()),
        NOW(),
        NULL
    FROM users u
    LEFT JOIN organizations o ON o.id = u.id
    WHERE o.id IS NULL
      AND u.deleted_at IS NULL
    ON CONFLICT (id) DO NOTHING;

    GET DIAGNOSTICS backfilled = ROW_COUNT;
    IF backfilled > 0 THEN
        RAISE NOTICE 'Backfilled % personal organizations', backfilled;
    END IF;

    -- Also create matching organization_members entries so the user shows up
    -- as OWNER of their personal org (matches what the handler-side onboarding
    -- already does for new accounts).
    INSERT INTO organization_members (org_id, user_id, role, created_at, updated_at)
    SELECT u.id, u.id, 'OWNER', NOW(), NOW()
    FROM users u
    LEFT JOIN organization_members m ON m.org_id = u.id AND m.user_id = u.id
    WHERE m.org_id IS NULL
      AND u.deleted_at IS NULL
    ON CONFLICT DO NOTHING;
END $$;

-- Now that every billing.org_id resolves to a real organizations row, add
-- the FKs. RESTRICT semantics: deleting an organization with active billing
-- is an operator action that should fail loudly.
ALTER TABLE transactions
    ADD CONSTRAINT fk_transactions_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE RESTRICT NOT VALID;

ALTER TABLE orders
    ADD CONSTRAINT fk_orders_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE RESTRICT NOT VALID;

ALTER TABLE subscriptions
    ADD CONSTRAINT fk_subscriptions_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE RESTRICT NOT VALID;

ALTER TABLE budgets
    ADD CONSTRAINT fk_budgets_org
    FOREIGN KEY (org_id) REFERENCES organizations(id) ON DELETE RESTRICT NOT VALID;

-- Validate so the constraints are enforced immediately on small tables.
-- Production operators with very large billing tables can run VALIDATE
-- during a low-traffic window if it's too slow.
ALTER TABLE transactions  VALIDATE CONSTRAINT fk_transactions_org;
ALTER TABLE orders        VALIDATE CONSTRAINT fk_orders_org;
ALTER TABLE subscriptions VALIDATE CONSTRAINT fk_subscriptions_org;
ALTER TABLE budgets       VALIDATE CONSTRAINT fk_budgets_org;
