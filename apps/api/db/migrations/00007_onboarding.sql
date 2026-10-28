-- +goose Up
-- Setup progress, one row per workspace. The current step is computed from these
-- columns and whether a WhatsApp number is connected.
CREATE TABLE workspace_onboarding (
    workspace_id         uuid        PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    first_goal           text        NOT NULL CHECK (first_goal IN ('offers', 'support', 'automate')),
    sample_data          boolean     NOT NULL DEFAULT false,
    business_done_at     timestamptz,
    whatsapp_skipped_at  timestamptz,
    test_message_sent_at timestamptz,
    -- Setup finished or skipped: the dashboard opens from now on.
    completed_at         timestamptz,
    -- Abandoned-setup reminder emails sent so far (at most 2).
    reminder_count       int         NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER workspace_onboarding_set_updated_at BEFORE UPDATE ON workspace_onboarding
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Per-member UI state: each person sees the tour and checklist on their own.
CREATE TABLE onboarding_member_state (
    workspace_id        uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id             uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tour_seen_at        timestamptz,
    checklist_hidden_at timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE TRIGGER onboarding_member_state_set_updated_at BEFORE UPDATE ON onboarding_member_state
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE onboarding_member_state;
DROP TABLE workspace_onboarding;
