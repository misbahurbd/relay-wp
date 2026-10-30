-- +goose Up
-- A resource a plan leaves out is now not included, where before it meant
-- unlimited (ADR 0008). Every plan keeps what it gave: a resource it had no
-- key for, among those limited until now plus contacts (enforced from here
-- on), becomes an explicit -1. Staff then set real contact limits.
UPDATE plans
SET limits = jsonb_build_object('members', -1, 'whatsapp_numbers', -1, 'contacts', -1) || limits;

-- +goose Down
SELECT 1;
