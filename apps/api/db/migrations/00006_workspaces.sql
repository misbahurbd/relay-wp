-- +goose Up
-- A business using Relay. Everything tenant-owned hangs off a workspace.
CREATE TABLE workspaces (
    id            uuid        PRIMARY KEY,
    name          text        NOT NULL,
    industry      text        CHECK (industry IN ('retail_ecommerce', 'food_restaurants', 'education',
                                                  'health_clinics', 'travel', 'real_estate', 'other')),
    website       text,
    -- ISO 3166-1 alpha-2
    country       text        CHECK (country ~ '^[A-Z]{2}$'),
    team_size     text        CHECK (team_size IN ('just_me', '2_5', '6_20', '20_plus')),
    goals         text[]      NOT NULL DEFAULT '{}'
                              CHECK (goals <@ ARRAY['marketing', 'support', 'sales']::text[]),
    plan          text        NOT NULL DEFAULT 'growth',
    trial_ends_at timestamptz,
    -- First real customer message sent; switches the dashboard from first-run to everyday.
    activated_at  timestamptz,
    created_by    uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER workspaces_set_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Who belongs to a workspace. Only the owner role exists until team access is built.
CREATE TABLE workspace_members (
    workspace_id   uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id        uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role           text        NOT NULL CHECK (role IN ('owner')),
    -- Picks the workspace to open after sign-in.
    last_active_at timestamptz NOT NULL DEFAULT now(),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id)
);
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id);
CREATE TRIGGER workspace_members_set_updated_at BEFORE UPDATE ON workspace_members
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE workspace_members;
DROP TABLE workspaces;
