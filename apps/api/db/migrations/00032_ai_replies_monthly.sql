-- +goose Up
-- AI replies become a monthly quota instead of an included feature (ADR 0008):
-- each reply costs Relay an AI call. An included feature was stored as 1, which
-- would now read as one reply a month, so every plan and grant override that
-- had it keeps it as unlimited (-1). Staff then set real monthly caps.
UPDATE plans
SET limits = jsonb_set(limits, '{ai_replies}', '-1')
WHERE limits -> 'ai_replies' = '1';

UPDATE plan_grants
SET overrides = jsonb_set(overrides, '{ai_replies}', '-1')
WHERE overrides -> 'ai_replies' = '1';

-- +goose Down
UPDATE plans
SET limits = jsonb_set(limits, '{ai_replies}', '1')
WHERE (limits ->> 'ai_replies')::int <> 0;

UPDATE plan_grants
SET overrides = jsonb_set(overrides, '{ai_replies}', '1')
WHERE (overrides ->> 'ai_replies')::int <> 0;
