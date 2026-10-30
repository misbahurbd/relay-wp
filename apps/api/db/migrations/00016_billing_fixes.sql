-- +goose Up
-- Billing fixes found walking the full flow (Phase 7).

-- Why a grant is suspended, so each cause is lifted only by its own cure: a
-- renewal restores a past-due suspension, a paid installment restores an
-- installments one, and only support restores a lost dispute.
ALTER TABLE plan_grants
    ADD COLUMN suspend_reason text CHECK (suspend_reason IN ('past_due', 'installments', 'dispute'));
UPDATE plan_grants
SET suspend_reason = CASE WHEN source = 'order' THEN 'installments' ELSE 'past_due' END
WHERE status = 'suspended';
ALTER TABLE plan_grants
    ADD CONSTRAINT plan_grants_suspended_has_reason_check CHECK ((status = 'suspended') = (suspend_reason IS NOT NULL));

-- Staff grants with an end date now expire like trials do.
DROP INDEX plan_grants_trial_end_idx;
CREATE INDEX plan_grants_expiry_idx ON plan_grants (ends_at)
    WHERE source IN ('trial', 'manual') AND status IN ('active', 'suspended') AND ends_at IS NOT NULL;

-- The past-due sweep: subscriptions whose grace has run out.
CREATE INDEX subscriptions_grace_idx ON subscriptions (grace_ends_at) WHERE status = 'past_due';

-- +goose Down
DROP INDEX subscriptions_grace_idx;
DROP INDEX plan_grants_expiry_idx;
CREATE INDEX plan_grants_trial_end_idx ON plan_grants (ends_at) WHERE source = 'trial' AND status = 'active';
ALTER TABLE plan_grants
    DROP CONSTRAINT plan_grants_suspended_has_reason_check,
    DROP COLUMN suspend_reason;
