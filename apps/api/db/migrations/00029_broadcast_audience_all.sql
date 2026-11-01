-- +goose Up
-- An audience names "everyone" explicitly now; before, no tags meant everyone (ADR 0013, BC-6).
UPDATE broadcasts SET audience = audience || '{"all": true}'
WHERE jsonb_array_length(COALESCE(audience -> 'tag_ids', '[]')) = 0;

-- +goose Down
UPDATE broadcasts SET audience = audience - 'all';
