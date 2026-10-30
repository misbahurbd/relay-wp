-- +goose Up
-- Plan resources (ADR 0008). Limits are live: a grant reads its plan's limits
-- as they are now, plus its own overrides, instead of a copy made at purchase.
-- A plan version's limits may now be edited in place; its prices still may not.
-- The free plan becomes a row so staff can edit its limits like any other.

ALTER TABLE plan_grants ADD COLUMN overrides jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(overrides) = 'object');

-- Keep what each grant had: only the keys that differ from its plan become overrides.
-- A key the plan has but the grant lacked was unlimited on the grant.
UPDATE plan_grants g
SET overrides = COALESCE((
    SELECT jsonb_object_agg(k, v)
    FROM (
        SELECT e.key AS k, e.value AS v FROM jsonb_each(g.limits) e WHERE p.limits -> e.key IS DISTINCT FROM e.value
        UNION ALL
        SELECT e.key, '-1'::jsonb FROM jsonb_each(p.limits) e WHERE NOT g.limits ? e.key
    ) d
), '{}')
FROM plans p
WHERE p.id = g.plan_id;

ALTER TABLE plan_grants DROP COLUMN limits;

INSERT INTO plans (code, version, name, limits, sort_order)
VALUES ('free', 1, 'Free', '{"members": 1, "whatsapp_numbers": 1}', 0);

-- Who changed a plan's limits or a grant's overrides, and from what. An edit
-- reaches live customers, so every one is kept.
CREATE TABLE limit_changes (
    id         uuid        PRIMARY KEY DEFAULT uuidv7(),
    plan_id    uuid        REFERENCES plans (id) ON DELETE RESTRICT,
    grant_id   uuid        REFERENCES plan_grants (id) ON DELETE RESTRICT,
    changed_by uuid        REFERENCES users (id) ON DELETE SET NULL,
    before     jsonb       NOT NULL,
    after      jsonb       NOT NULL,
    reason     text        NOT NULL CHECK (reason <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((plan_id IS NULL) <> (grant_id IS NULL))
);
CREATE INDEX limit_changes_plan_idx ON limit_changes (plan_id, created_at DESC) WHERE plan_id IS NOT NULL;
CREATE INDEX limit_changes_grant_idx ON limit_changes (grant_id, created_at DESC) WHERE grant_id IS NOT NULL;

-- +goose Down
DROP TABLE limit_changes;
DELETE FROM plans WHERE code = 'free';
ALTER TABLE plan_grants ADD COLUMN limits jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(limits) = 'object');
UPDATE plan_grants g SET limits = p.limits || g.overrides FROM plans p WHERE p.id = g.plan_id;
ALTER TABLE plan_grants ALTER COLUMN limits DROP DEFAULT;
ALTER TABLE plan_grants DROP COLUMN overrides;
