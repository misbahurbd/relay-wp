-- +goose Up
-- The people a workspace talks to on WhatsApp (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-2, AD-3).
-- A contact is identified by its E.164 phone within the workspace.
CREATE TABLE contacts (
    id              uuid        PRIMARY KEY,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name            text        NOT NULL DEFAULT '',
    -- E.164, for example +447700900412.
    phone           text        NOT NULL,
    email           citext,
    opt_in          text        NOT NULL DEFAULT 'pending' CHECK (opt_in IN ('in', 'out', 'pending')),
    opt_in_source   text,
    opt_in_at       timestamptz,
    source          text        NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'import', 'inbox', 'api')),
    language        text,
    -- Custom fields: string values only, for example {"City": "Leeds"}.
    attributes      jsonb       NOT NULL DEFAULT '{}',
    assigned_to     uuid        REFERENCES users (id) ON DELETE SET NULL,
    -- Filled by the Inbox once it exists.
    last_message_at timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT contacts_workspace_phone_key UNIQUE (workspace_id, phone)
);
-- Paging is id DESC (UUIDv7, so newest first).
CREATE INDEX contacts_workspace_id_idx ON contacts (workspace_id, id DESC);
CREATE INDEX contacts_workspace_opt_in_idx ON contacts (workspace_id, opt_in);
CREATE INDEX contacts_workspace_last_message_idx ON contacts (workspace_id, last_message_at);
CREATE INDEX contacts_assigned_to_idx ON contacts (assigned_to) WHERE assigned_to IS NOT NULL;
CREATE TRIGGER contacts_set_updated_at BEFORE UPDATE ON contacts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE contact_tags (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         citext      NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT contact_tags_workspace_name_key UNIQUE (workspace_id, name)
);

CREATE TABLE contact_tag_links (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    contact_id   uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    tag_id       uuid NOT NULL REFERENCES contact_tags (id) ON DELETE CASCADE,
    PRIMARY KEY (contact_id, tag_id)
);
CREATE INDEX contact_tag_links_tag_idx ON contact_tag_links (tag_id);

-- A contacts file being imported (AD-5): uploaded, then mapped, then worked by a River job.
CREATE TABLE contact_imports (
    id            uuid        PRIMARY KEY,
    workspace_id  uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    created_by    uuid        REFERENCES users (id) ON DELETE SET NULL,
    file_name     text        NOT NULL,
    file_type     text        NOT NULL CHECK (file_type IN ('csv', 'xlsx')),
    -- Storage key of the uploaded file.
    file_key      text        NOT NULL,
    row_count     integer     NOT NULL,
    -- The file's column names, in order.
    columns       jsonb       NOT NULL DEFAULT '[]',
    -- [{name, field, attribute_key}] once mapped.
    mapping       jsonb,
    tag_name      text,
    status        text        NOT NULL DEFAULT 'uploaded'
                              CHECK (status IN ('uploaded', 'mapped', 'importing', 'done', 'failed')),
    created_count integer     NOT NULL DEFAULT 0,
    updated_count integer     NOT NULL DEFAULT 0,
    skipped_count integer     NOT NULL DEFAULT 0,
    -- Storage key of the skipped-rows CSV, when any row was skipped.
    skipped_key   text,
    error         text,
    started_at    timestamptz,
    finished_at   timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contact_imports_workspace_idx ON contact_imports (workspace_id, created_at);
CREATE TRIGGER contact_imports_set_updated_at BEFORE UPDATE ON contact_imports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE contact_imports;
DROP TABLE contact_tag_links;
DROP TABLE contact_tags;
DROP TABLE contacts;
