-- +goose Up
-- Team access (docs/PHASE-3-TEAM-ACCESS.md): roles, per-member overrides, teams,
-- WhatsApp number reach, invitations and the access log.

-- The permission catalog. Mirrors shared/authctx/permissions.go so foreign keys
-- hold. A bit is never reused: a retired permission keeps its row.
CREATE TABLE permissions (
    key       text     PRIMARY KEY,
    bit       smallint NOT NULL UNIQUE CHECK (bit BETWEEN 0 AND 62),
    group_key text     NOT NULL,
    label     text     NOT NULL,
    sensitive boolean  NOT NULL DEFAULT false,
    position  int      NOT NULL
);
INSERT INTO permissions (key, bit, group_key, label, sensitive, position) VALUES
    ('inbox.reply',           0,  'inbox',        'Reply to customers',                false, 10),
    ('inbox.send_template',   1,  'inbox',        'Send templates outside 24h window', false, 11),
    ('inbox.close_reassign',  2,  'inbox',        'Close and reassign chats',          false, 12),
    ('inbox.notes',           3,  'inbox',        'Add private notes',                 false, 13),
    ('contacts.view',         4,  'contacts',     'View contacts',                     false, 20),
    ('contacts.edit',         5,  'contacts',     'Edit contacts',                     false, 21),
    ('contacts.import',       6,  'contacts',     'Import contacts',                   false, 22),
    ('contacts.export',       7,  'contacts',     'Export contacts',                   true,  23),
    ('contacts.delete',       8,  'contacts',     'Delete contacts',                   true,  24),
    ('broadcasts.create',     9,  'broadcasts',   'Create broadcasts',                 false, 30),
    ('broadcasts.send',       10, 'broadcasts',   'Send broadcasts',                   false, 31),
    ('broadcasts.reports',    11, 'broadcasts',   'View reports',                      false, 32),
    ('templates.create',      12, 'templates',    'Create templates',                  false, 40),
    ('templates.submit',      13, 'templates',    'Submit to Meta',                    false, 41),
    ('templates.delete',      14, 'templates',    'Delete templates',                  false, 42),
    ('automations.edit',      15, 'automations',  'Edit flows and automations',        false, 50),
    ('automations.publish',   16, 'automations',  'Publish changes',                   false, 51),
    ('integrations.manage',   17, 'integrations', 'Manage integrations',               false, 60),
    ('integrations.api_keys', 18, 'integrations', 'View and create API keys',          true,  61),
    ('team.invite',           19, 'team',         'Invite members',                    false, 70),
    ('team.roles',            20, 'team',         'Change roles and access',           true,  71),
    ('team.remove',           21, 'team',         'Remove and suspend members',        false, 72),
    ('billing.view',          22, 'billing',      'View billing',                      false, 80),
    ('billing.pay',           23, 'billing',      'Pay and change plan',               false, 81),
    ('billing.topup',         24, 'billing',      'Top up wallet',                     false, 82);

-- Seats on the plan. Owner, Admin, Agent and custom roles use one; Viewers are free.
ALTER TABLE workspaces ADD COLUMN seat_limit int NOT NULL DEFAULT 5 CHECK (seat_limit >= 1);

-- Every workspace has its own copy of the four system roles, plus custom ones.
CREATE TABLE roles (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    -- owner, admin, agent, viewer; NULL for a custom role.
    system_key   text        CHECK (system_key IN ('owner', 'admin', 'agent', 'viewer')),
    name         text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    description  text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    color        text        NOT NULL DEFAULT 'blue' CHECK (color IN ('green', 'blue', 'amber', 'red', 'gray')),
    chat_scope   text        NOT NULL CHECK (chat_scope IN ('assigned', 'team', 'all')),
    uses_seat    boolean     NOT NULL DEFAULT true,
    locked       boolean     NOT NULL DEFAULT false,
    -- Bumped with every change to the role or its permissions (optimistic concurrency).
    version      int         NOT NULL DEFAULT 1,
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id)
);
CREATE UNIQUE INDEX roles_system_key_idx ON roles (workspace_id, system_key) WHERE system_key IS NOT NULL;
CREATE UNIQUE INDEX roles_name_idx ON roles (workspace_id, lower(name));
CREATE TRIGGER roles_set_updated_at BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE role_permissions (
    workspace_id   uuid NOT NULL,
    role_id        uuid NOT NULL,
    permission_key text NOT NULL REFERENCES permissions (key),
    PRIMARY KEY (role_id, permission_key),
    FOREIGN KEY (workspace_id, role_id) REFERENCES roles (workspace_id, id) ON DELETE CASCADE
);

-- System roles for workspaces created before this migration. New workspaces
-- get them from features/team in the create-workspace transaction.
INSERT INTO roles (id, workspace_id, system_key, name, description, color, chat_scope, uses_seat, locked)
SELECT uuidv7(), w.id, r.key, r.name, r.description, r.color, r.scope, r.key <> 'viewer', r.key = 'owner'
FROM workspaces w
CROSS JOIN (VALUES
    ('owner',  'Owner',  'Full access, including billing and deleting the workspace', 'green', 'all'),
    ('admin',  'Admin',  'Manages everything except paying and changing the plan',     'blue',  'all'),
    ('agent',  'Agent',  'Answers customers in the inbox',                             'amber', 'team'),
    ('viewer', 'Viewer', 'Read-only access to chats and reports',                      'gray',  'team')
) AS r (key, name, description, color, scope);

INSERT INTO role_permissions (workspace_id, role_id, permission_key)
SELECT r.workspace_id, r.id, p.key
FROM roles r
JOIN permissions p ON
    (r.system_key = 'owner')
    OR (r.system_key = 'admin' AND p.key <> 'billing.pay')
    OR (r.system_key = 'agent' AND p.key IN ('inbox.reply', 'inbox.send_template', 'inbox.close_reassign',
                                              'inbox.notes', 'contacts.view', 'contacts.edit', 'broadcasts.reports'))
    OR (r.system_key = 'viewer' AND p.key IN ('contacts.view', 'broadcasts.reports'));

-- Members point at a role instead of holding a role name.
ALTER TABLE workspace_members
    ADD COLUMN role_id        uuid,
    ADD COLUMN status         text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
    -- NULL means the role's scope.
    ADD COLUMN chat_scope     text        CHECK (chat_scope IN ('assigned', 'team', 'all')),
    ADD COLUMN availability   text        NOT NULL DEFAULT 'offline' CHECK (availability IN ('online', 'away', 'offline')),
    -- Bumped with every change to this member's role, overrides, teams, numbers, scope or status.
    ADD COLUMN access_version int         NOT NULL DEFAULT 1,
    ADD COLUMN invited_by     uuid        REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN joined_at      timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN suspended_at   timestamptz,
    ADD COLUMN suspended_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN suspend_reason text        CHECK (char_length(suspend_reason) <= 200);
UPDATE workspace_members m
SET role_id = r.id, joined_at = m.created_at
FROM roles r
WHERE r.workspace_id = m.workspace_id AND r.system_key = m.role;
ALTER TABLE workspace_members
    ALTER COLUMN role_id SET NOT NULL,
    ADD FOREIGN KEY (workspace_id, role_id) REFERENCES roles (workspace_id, id),
    DROP COLUMN role;
CREATE INDEX workspace_members_role_idx ON workspace_members (workspace_id, role_id);

-- Extra permissions for one person (grant), or one taken away (deny), optionally until a date.
CREATE TABLE member_permission_overrides (
    id             uuid        PRIMARY KEY,
    workspace_id   uuid        NOT NULL,
    user_id        uuid        NOT NULL,
    permission_key text        NOT NULL REFERENCES permissions (key),
    effect         text        NOT NULL CHECK (effect IN ('grant', 'deny')),
    reason         text        NOT NULL DEFAULT '' CHECK (char_length(reason) <= 200),
    granted_by     uuid        REFERENCES users (id) ON DELETE SET NULL,
    expires_at     timestamptz,
    -- Set by the 24-hour warning job.
    expiry_notified_at timestamptz,
    revoked_at     timestamptz,
    revoked_by     uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);
CREATE INDEX member_overrides_member_idx ON member_permission_overrides (workspace_id, user_id) WHERE revoked_at IS NULL;
CREATE INDEX member_overrides_expiry_idx ON member_permission_overrides (expires_at)
    WHERE revoked_at IS NULL AND expires_at IS NOT NULL;
CREATE UNIQUE INDEX member_overrides_active_idx ON member_permission_overrides (workspace_id, user_id, permission_key)
    WHERE revoked_at IS NULL;
CREATE TRIGGER member_permission_overrides_set_updated_at BEFORE UPDATE ON member_permission_overrides
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Which WhatsApp numbers a member works on.
CREATE TABLE member_numbers (
    workspace_id uuid        NOT NULL,
    user_id      uuid        NOT NULL,
    number_id    uuid        NOT NULL REFERENCES whatsapp_numbers (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id, number_id),
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);

CREATE TABLE teams (
    id             uuid        PRIMARY KEY,
    workspace_id   uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name           text        NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    description    text        NOT NULL DEFAULT '' CHECK (char_length(description) <= 200),
    color          text        NOT NULL DEFAULT 'blue' CHECK (color IN ('green', 'blue', 'amber', 'red', 'gray')),
    number_id      uuid        REFERENCES whatsapp_numbers (id) ON DELETE SET NULL,
    routing_method text        NOT NULL DEFAULT 'round_robin' CHECK (routing_method IN ('round_robin', 'least_busy', 'manual')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, id)
);
CREATE UNIQUE INDEX teams_name_idx ON teams (workspace_id, lower(name));
CREATE TRIGGER teams_set_updated_at BEFORE UPDATE ON teams
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE team_members (
    workspace_id uuid        NOT NULL,
    team_id      uuid        NOT NULL,
    user_id      uuid        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id),
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);
CREATE INDEX team_members_member_idx ON team_members (workspace_id, user_id);

-- Invitations by email. The token is stored hashed and read only by GetInvitationByTokenHash.
CREATE TABLE invitations (
    id                   uuid        PRIMARY KEY,
    workspace_id         uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    email                citext      NOT NULL,
    role_id              uuid        NOT NULL,
    -- NULL means the role's scope.
    chat_scope           text        CHECK (chat_scope IN ('assigned', 'team', 'all')),
    token_hash           bytea       NOT NULL UNIQUE,
    personal_message     text        NOT NULL DEFAULT '' CHECK (char_length(personal_message) <= 500),
    require_2fa          boolean     NOT NULL DEFAULT false,
    -- Expired = pending past expires_at.
    status               text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked')),
    invited_by           uuid        REFERENCES users (id) ON DELETE SET NULL,
    send_count           int         NOT NULL DEFAULT 1,
    last_sent_at         timestamptz NOT NULL DEFAULT now(),
    expires_at           timestamptz NOT NULL,
    accepted_at          timestamptz,
    accepted_user_id     uuid        REFERENCES users (id) ON DELETE SET NULL,
    revoked_at           timestamptz,
    revoked_by           uuid        REFERENCES users (id) ON DELETE SET NULL,
    renewal_requested_at timestamptz,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, role_id) REFERENCES roles (workspace_id, id),
    UNIQUE (workspace_id, id)
);
CREATE UNIQUE INDEX invitations_pending_email_idx ON invitations (workspace_id, email) WHERE status = 'pending';
CREATE INDEX invitations_status_idx ON invitations (workspace_id, status);
CREATE TRIGGER invitations_set_updated_at BEFORE UPDATE ON invitations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE invitation_teams (
    workspace_id  uuid NOT NULL,
    invitation_id uuid NOT NULL,
    team_id       uuid NOT NULL,
    PRIMARY KEY (invitation_id, team_id),
    FOREIGN KEY (workspace_id, invitation_id) REFERENCES invitations (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE invitation_numbers (
    workspace_id  uuid NOT NULL,
    invitation_id uuid NOT NULL,
    number_id     uuid NOT NULL REFERENCES whatsapp_numbers (id) ON DELETE CASCADE,
    PRIMARY KEY (invitation_id, number_id),
    FOREIGN KEY (workspace_id, invitation_id) REFERENCES invitations (workspace_id, id) ON DELETE CASCADE
);

CREATE TABLE invitation_permission_overrides (
    workspace_id   uuid NOT NULL,
    invitation_id  uuid NOT NULL,
    permission_key text NOT NULL REFERENCES permissions (key),
    effect         text NOT NULL CHECK (effect IN ('grant', 'deny')),
    PRIMARY KEY (invitation_id, permission_key),
    FOREIGN KEY (workspace_id, invitation_id) REFERENCES invitations (workspace_id, id) ON DELETE CASCADE
);

-- Who changed what, for whom, and when. Append-only: rows leave only when a
-- monthly partition older than 12 months is dropped. No foreign keys, so the
-- log outlives removed members and deleted users.
CREATE TABLE access_log_events (
    id              uuid        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    workspace_id    uuid        NOT NULL,
    -- NULL for the system (an expired permission).
    actor_user_id   uuid,
    -- NULL for an invite that hasn't been accepted.
    subject_user_id uuid,
    subject_email   citext,
    action          text        NOT NULL,
    target          text        NOT NULL DEFAULT '',
    before_value    jsonb,
    after_value     jsonb,
    note            text        NOT NULL DEFAULT '',
    ip              inet,
    user_agent      text        NOT NULL DEFAULT '',
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);
CREATE INDEX access_log_workspace_idx ON access_log_events (workspace_id, created_at DESC, id DESC);
CREATE INDEX access_log_subject_idx ON access_log_events (workspace_id, subject_user_id, created_at DESC);
CREATE TABLE access_log_events_default PARTITION OF access_log_events DEFAULT;

-- This month and the next two; the access_log_partitions job keeps two months ahead.
-- +goose StatementBegin
DO $$
DECLARE m date := date_trunc('month', now());
BEGIN
  FOR i IN 0..2 LOOP
    EXECUTE format('CREATE TABLE IF NOT EXISTS access_log_events_%s PARTITION OF access_log_events FOR VALUES FROM (%L) TO (%L)',
                   to_char(m, 'YYYY_MM'), m, m + interval '1 month');
    m := m + interval '1 month';
  END LOOP;
END $$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE FUNCTION access_log_append_only() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'access_log_events is append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER access_log_events_append_only BEFORE UPDATE OR DELETE ON access_log_events
    FOR EACH ROW EXECUTE FUNCTION access_log_append_only();

-- +goose Down
DROP TABLE access_log_events;
DROP FUNCTION access_log_append_only();
DROP TABLE invitation_permission_overrides;
DROP TABLE invitation_numbers;
DROP TABLE invitation_teams;
DROP TABLE invitations;
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE member_numbers;
DROP TABLE member_permission_overrides;
-- Members of custom roles fall back to nothing; only owners existed before this.
DELETE FROM workspace_members m USING roles r
WHERE r.id = m.role_id AND r.system_key IS DISTINCT FROM 'owner';
ALTER TABLE workspace_members ADD COLUMN role text;
UPDATE workspace_members SET role = 'owner';
ALTER TABLE workspace_members
    ALTER COLUMN role SET NOT NULL,
    ADD CONSTRAINT workspace_members_role_check CHECK (role IN ('owner')),
    DROP COLUMN role_id,
    DROP COLUMN status,
    DROP COLUMN chat_scope,
    DROP COLUMN availability,
    DROP COLUMN access_version,
    DROP COLUMN invited_by,
    DROP COLUMN joined_at,
    DROP COLUMN suspended_at,
    DROP COLUMN suspended_by,
    DROP COLUMN suspend_reason;
DROP TABLE role_permissions;
DROP TABLE roles;
ALTER TABLE workspaces DROP COLUMN seat_limit;
DROP TABLE permissions;
