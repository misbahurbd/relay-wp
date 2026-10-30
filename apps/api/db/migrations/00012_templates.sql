-- +goose Up
-- WhatsApp message templates (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-6). Drafts live
-- only here; submitted ones mirror a template on the workspace's WABA.
CREATE TABLE message_templates (
    id                uuid        PRIMARY KEY,
    workspace_id      uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    -- The WABA it was submitted to; NULL while a draft.
    account_id        uuid        REFERENCES whatsapp_accounts (id) ON DELETE SET NULL,
    name              text        NOT NULL CHECK (name ~ '^[a-z0-9_]+$' AND char_length(name) <= 512),
    -- Meta's language code, e.g. en_US.
    language          text        NOT NULL CHECK (char_length(language) BETWEEN 2 AND 15),
    category          text        NOT NULL CHECK (category IN ('marketing', 'utility', 'authentication')),
    status            text        NOT NULL DEFAULT 'draft'
                                  CHECK (status IN ('draft', 'pending', 'approved', 'rejected', 'paused', 'disabled')),
    meta_template_id  text,
    rejected_reason   text,
    -- Our own shape {header, body, footer, buttons}; converted to Meta's only at the boundary.
    components        jsonb       NOT NULL DEFAULT '{}',
    -- Storage key, content type and size of the uploaded media header sample.
    header_media_key  text,
    header_media_type text,
    header_media_size bigint,
    submitted_at      timestamptz,
    created_by        uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT message_templates_name_language_key UNIQUE (workspace_id, name, language)
);
-- Paging (id DESC) and the list filters.
CREATE INDEX message_templates_workspace_idx ON message_templates (workspace_id, id DESC);
CREATE INDEX message_templates_status_idx ON message_templates (workspace_id, status, id DESC);
CREATE INDEX message_templates_category_idx ON message_templates (workspace_id, category, id DESC);
-- Status webhooks find the row by Meta's template ID.
CREATE INDEX message_templates_meta_idx ON message_templates (workspace_id, meta_template_id)
    WHERE meta_template_id IS NOT NULL;
CREATE TRIGGER message_templates_set_updated_at BEFORE UPDATE ON message_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose Down
DROP TABLE message_templates;
