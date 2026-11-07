-- +goose Up
-- A grant is suspended while any cause holds, and each cause is added and
-- lifted on its own: a renewal lifts past_due, a paid installment lifts
-- installments, and only support lifts a lost dispute. One reason column let
-- a later cause overwrite or skip an earlier one.
ALTER TABLE plan_grants
    ADD COLUMN suspend_reasons text[] NOT NULL DEFAULT '{}'
        CHECK (suspend_reasons <@ ARRAY['past_due', 'installments', 'dispute']::text[]);
UPDATE plan_grants
SET suspend_reasons = ARRAY[suspend_reason]
WHERE suspend_reason IS NOT NULL;
ALTER TABLE plan_grants
    DROP CONSTRAINT plan_grants_suspended_has_reason_check,
    DROP COLUMN suspend_reason,
    ADD CONSTRAINT plan_grants_suspended_has_reason_check
        CHECK ((status = 'suspended') = (cardinality(suspend_reasons) > 0));

-- +goose Down
ALTER TABLE plan_grants
    ADD COLUMN suspend_reason text CHECK (suspend_reason IN ('past_due', 'installments', 'dispute'));
UPDATE plan_grants
SET suspend_reason = CASE
                         WHEN 'dispute' = ANY (suspend_reasons) THEN 'dispute'
                         ELSE suspend_reasons[1]
                     END
WHERE cardinality(suspend_reasons) > 0;
ALTER TABLE plan_grants
    DROP CONSTRAINT plan_grants_suspended_has_reason_check,
    DROP COLUMN suspend_reasons,
    ADD CONSTRAINT plan_grants_suspended_has_reason_check CHECK ((status = 'suspended') = (suspend_reason IS NOT NULL));
