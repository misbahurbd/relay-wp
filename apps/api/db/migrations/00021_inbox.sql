-- +goose Up
-- Team Inbox, slice A (docs/PHASE-7-INBOX.md AD-2, AD-4, section 5): one thread per
-- contact and channel, its messages, quick replies and note mentions. The data is
-- channel-neutral (ADR 0010).

-- One thread per contact and channel account (AD-2). The last-message fields are
-- denormalized here and written in the same transaction as each message, so the
-- list never reads messages.
CREATE TABLE conversations (
    id                     uuid        PRIMARY KEY,
    workspace_id           uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    contact_id             uuid        NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    channel_type           text        NOT NULL CHECK (channel_type IN ('whatsapp', 'messenger', 'instagram')),
    -- The provider's account row: whatsapp_numbers.id for WhatsApp. No FK, because the
    -- reference is polymorphic (channel_type says which table) and a chat outlives a
    -- disconnected channel (ADR 0010).
    channel_id             uuid        NOT NULL,
    status                 text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'pending', 'closed')),
    team_id                uuid,
    assignee_id            uuid,
    -- From the referral on the first message.
    source                 text        NOT NULL DEFAULT 'organic' CHECK (source IN ('organic', 'ctwa_ad', 'template')),
    last_message_at        timestamptz NOT NULL DEFAULT now(),
    last_message_preview   text        NOT NULL DEFAULT '' CHECK (char_length(last_message_preview) <= 120),
    last_message_direction text        NOT NULL DEFAULT 'in' CHECK (last_message_direction IN ('in', 'out')),
    last_inbound_at        timestamptz,
    -- last_inbound_at + the channel's window: free-form replies are allowed until then
    -- (AD-6). NULL for a channel without a window.
    window_expires_at      timestamptz,
    unread_count           int         NOT NULL DEFAULT 0 CHECK (unread_count >= 0),
    -- The first inbound message not yet answered; cleared by an outbound reply.
    waiting_since          timestamptz,
    snoozed_until          timestamptz,
    closed_at              timestamptz,
    created_at             timestamptz NOT NULL DEFAULT now(),
    updated_at             timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT conversations_contact_channel_key UNIQUE (workspace_id, contact_id, channel_type, channel_id),
    UNIQUE (workspace_id, id),
    FOREIGN KEY (workspace_id, team_id) REFERENCES teams (workspace_id, id) ON DELETE SET NULL (team_id),
    -- A removed member's chats go back to the queue.
    FOREIGN KEY (workspace_id, assignee_id) REFERENCES workspace_members (workspace_id, user_id)
        ON DELETE SET NULL (assignee_id),
    CHECK (snoozed_until IS NULL OR status = 'pending')
);
CREATE INDEX conversations_status_idx ON conversations (workspace_id, status, last_message_at DESC, id);
CREATE INDEX conversations_assignee_idx ON conversations (workspace_id, assignee_id, status, last_message_at DESC);
CREATE INDEX conversations_team_idx ON conversations (workspace_id, team_id, status, last_message_at DESC);
CREATE INDEX conversations_snoozed_idx ON conversations (snoozed_until) WHERE snoozed_until IS NOT NULL;
CREATE INDEX conversations_contact_idx ON conversations (contact_id);
CREATE INDEX conversations_channel_idx ON conversations (workspace_id, channel_type, channel_id);
CREATE TRIGGER conversations_set_updated_at BEFORE UPDATE ON conversations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Every message, note and system event of a conversation (AD-4). id is UUIDv7,
-- so it orders the thread and is its cursor.
CREATE TABLE messages (
    id              uuid        PRIMARY KEY,
    workspace_id    uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    conversation_id uuid        NOT NULL,
    direction       text        NOT NULL CHECK (direction IN ('in', 'out')),
    kind            text        NOT NULL DEFAULT 'message' CHECK (kind IN ('message', 'note', 'event')),
    -- The channel's type (text, image, …, template, unsupported); notes are text, events are event.
    type            text        NOT NULL CHECK (char_length(type) BETWEEN 1 AND 32),
    -- Text, caption, or the rendered template.
    body            text        NOT NULL DEFAULT '',
    -- Our own shape per type: media, location, contacts, interactive, template or event.
    content         jsonb       NOT NULL DEFAULT '{}',
    -- The channel's message ID (WhatsApp's wamid); NULL for notes, events and messages
    -- not yet sent. Ingest and status updates match on it (AD-3, AD-7).
    external_id     text,
    reply_to_id     uuid        REFERENCES messages (id) ON DELETE SET NULL,
    -- Outbound messages only. Moves forward only (AD-7).
    status          text        CHECK (status IN ('pending', 'sent', 'delivered', 'read', 'failed')),
    error_code      text,
    error_title     text,
    -- The member who sent the message or wrote the note.
    author_id       uuid        REFERENCES users (id) ON DELETE SET NULL,
    mentions        uuid[]      NOT NULL DEFAULT '{}',
    -- {"in": emoji, "out": emoji}: one reaction per side, replaced or removed.
    reactions       jsonb       NOT NULL DEFAULT '{}',
    created_at      timestamptz NOT NULL DEFAULT now(),
    status_at       timestamptz,
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE CASCADE,
    CONSTRAINT messages_external_id_key UNIQUE (workspace_id, external_id),
    CHECK (kind = 'message' OR external_id IS NULL),
    CHECK (status IS NULL OR (direction = 'out' AND kind = 'message'))
);
CREATE INDEX messages_thread_idx ON messages (conversation_id, id DESC);
CREATE INDEX messages_notes_idx ON messages (workspace_id, author_id) WHERE kind = 'note';

-- Canned replies for the composer's "/" menu (F8).
CREATE TABLE quick_replies (
    id           uuid        PRIMARY KEY,
    workspace_id uuid        NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    shortcut     text        NOT NULL CHECK (shortcut ~ '^[a-z0-9_-]{1,32}$'),
    title        text        NOT NULL CHECK (char_length(title) BETWEEN 1 AND 80),
    body         text        NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1024),
    created_by   uuid        REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT quick_replies_shortcut_key UNIQUE (workspace_id, shortcut)
);
CREATE TRIGGER quick_replies_set_updated_at BEFORE UPDATE ON quick_replies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- A member @mentioned in a note (AD-12), for the bell and its list.
CREATE TABLE inbox_mentions (
    workspace_id    uuid        NOT NULL,
    message_id      uuid        NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    conversation_id uuid        NOT NULL,
    user_id         uuid        NOT NULL,
    seen_at         timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id),
    FOREIGN KEY (workspace_id, conversation_id) REFERENCES conversations (workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);
CREATE INDEX inbox_mentions_user_idx ON inbox_mentions (workspace_id, user_id, seen_at);

-- +goose Down
DROP TABLE inbox_mentions;
DROP TABLE quick_replies;
DROP TABLE messages;
DROP TABLE conversations;
