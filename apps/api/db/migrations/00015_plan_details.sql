-- +goose Up
-- What the pricing table says about a plan (docs/PHASE-7-BILLING.md): a one-line
-- description, the feature list, and whether it is the recommended plan. Plans
-- are never edited, so a new version carries its own copy.
ALTER TABLE plans
    ADD COLUMN description text   NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    ADD COLUMN features    text[] NOT NULL DEFAULT '{}' CHECK (cardinality(features) <= 20),
    ADD COLUMN recommended bool   NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE plans
    DROP COLUMN recommended,
    DROP COLUMN features,
    DROP COLUMN description;
