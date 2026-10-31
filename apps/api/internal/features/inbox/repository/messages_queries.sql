-- ---------- Messages (docs/PHASE-7-INBOX.md AD-3 to AD-7) ----------

-- name: InsertMessage :many
-- Any message, note or event. An external ID already stored inserts nothing and
-- returns no row (AD-3), so a webhook delivered twice is stored once.
INSERT INTO messages (id, workspace_id, conversation_id, direction, kind, type, body, content, external_id,
                      reply_to_id, status, author_id, mentions, created_at)
VALUES (@id, @workspace_id, @conversation_id, @direction, @kind, @type, @body, @content, sqlc.narg(external_id),
        sqlc.narg(reply_to_id), sqlc.narg(status), sqlc.narg(author_id), @mentions::uuid[], @created_at)
ON CONFLICT (workspace_id, external_id) DO NOTHING
RETURNING id;

-- name: ListMessages :many
-- One page of a thread, newest first. Cursor: the id of the oldest message so far.
SELECT m.id, m.direction, m.kind, m.type, m.body, m.content, m.reply_to_id, m.status, m.error_code,
       m.error_title, m.author_id, m.automation_run_id, m.mentions, m.reactions, m.created_at, m.status_at,
       u.email AS author_email, p.display_name AS author_display_name,
       p.first_name AS author_first_name, p.last_name AS author_last_name,
       r.direction AS reply_direction, r.type AS reply_type, r.body AS reply_body
FROM messages m
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN user_profiles p ON p.user_id = m.author_id
LEFT JOIN messages r ON r.id = m.reply_to_id
WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id
  AND (sqlc.narg(before_id)::uuid IS NULL OR m.id < sqlc.narg(before_id))
ORDER BY m.id DESC
LIMIT @page_size;

-- name: GetMessage :one
SELECT m.id, m.conversation_id, m.direction, m.kind, m.type, m.body, m.content, m.external_id, m.status
FROM messages m
WHERE m.workspace_id = @workspace_id AND m.id = @id;

-- name: MessageByExternalID :one
SELECT m.id, m.conversation_id
FROM messages m
WHERE m.workspace_id = @workspace_id AND m.external_id = @external_id;

-- name: GetMessageToSend :one
-- What the send job needs: the message, where it goes and the external ID it
-- quotes. The recipient is the contact's newest identity on the chat's channel
-- (the address the customer last wrote from), else their phone.
SELECT m.id, m.conversation_id, m.type, m.body, m.content, m.status, m.external_id,
       r.external_id AS reply_external_id, c.channel_type, c.channel_id,
       COALESCE((SELECT ci.external_id FROM contact_identities ci
                 WHERE ci.workspace_id = c.workspace_id AND ci.contact_id = c.contact_id
                   AND ci.channel_type = c.channel_type
                 ORDER BY ci.created_at DESC
                 LIMIT 1), ct.phone)::text AS recipient
FROM messages m
JOIN conversations c ON c.id = m.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN messages r ON r.id = m.reply_to_id
WHERE m.workspace_id = @workspace_id AND m.id = @id;

-- name: LastInboundExternalID :one
SELECT m.external_id
FROM messages m
WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id
  AND m.kind = 'message' AND m.direction = 'in' AND m.external_id IS NOT NULL
ORDER BY m.id DESC
LIMIT 1;

-- name: AdvanceMessageStatus :many
-- The channel's status webhook (AD-7): pending 0 < sent 1 < delivered 2 < read 3 < failed 4,
-- and the status only moves forward, so out-of-order webhooks can't undo a tick.
UPDATE messages
SET status      = @status::text,
    status_at   = @at::timestamptz,
    error_code  = CASE WHEN @status::text = 'failed' THEN sqlc.narg(error_code)::text ELSE messages.error_code END,
    error_title = CASE WHEN @status::text = 'failed' THEN sqlc.narg(error_title)::text ELSE messages.error_title END
FROM conversations c
WHERE messages.workspace_id = @workspace_id AND messages.external_id = @external_id::text AND messages.status IS NOT NULL
  AND c.id = messages.conversation_id
  AND (CASE messages.status WHEN 'pending' THEN 0 WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2 WHEN 'read' THEN 3 ELSE 4 END)
    < (CASE @status::text WHEN 'pending' THEN 0 WHEN 'sent' THEN 1 WHEN 'delivered' THEN 2 WHEN 'read' THEN 3 ELSE 4 END)
RETURNING messages.id, messages.conversation_id, c.team_id, c.assignee_id, c.channel_type, c.channel_id, c.contact_id;

-- name: MarkMessageSent :many
-- The send job got the channel's message ID. Only a pending message moves.
UPDATE messages
SET external_id = @external_id::text, status = 'sent', status_at = @at::timestamptz
FROM conversations c
WHERE messages.workspace_id = @workspace_id AND messages.id = @id AND messages.status = 'pending'
  AND c.id = messages.conversation_id
RETURNING messages.conversation_id, c.team_id, c.assignee_id, c.channel_type, c.channel_id;

-- name: MarkMessageFailed :many
UPDATE messages
SET status = 'failed', status_at = @at::timestamptz, error_code = @error_code::text, error_title = @error_title::text
FROM conversations c
WHERE messages.workspace_id = @workspace_id AND messages.id = @id AND messages.status = 'pending'
  AND c.id = messages.conversation_id
RETURNING messages.conversation_id, c.team_id, c.assignee_id, c.channel_type, c.channel_id;

-- name: SetReaction :many
-- One side's reaction on a message; an empty emoji removes it.
UPDATE messages
SET reactions = CASE WHEN @emoji::text = '' THEN messages.reactions - @side::text
                     ELSE jsonb_set(messages.reactions, ARRAY[@side::text], to_jsonb(@emoji::text)) END
FROM conversations c
WHERE messages.workspace_id = @workspace_id AND messages.id = @id AND c.id = messages.conversation_id
RETURNING messages.conversation_id, c.team_id, c.assignee_id, c.channel_type, c.channel_id;

-- name: SetMessageMedia :many
-- The fetched file's storage key, type, size and hash, merged into content.media.
UPDATE messages
SET content = jsonb_set(messages.content, '{media}', COALESCE(messages.content -> 'media', '{}'::jsonb) || @media::jsonb)
FROM conversations c
WHERE messages.workspace_id = @workspace_id AND messages.id = @id AND c.id = messages.conversation_id
RETURNING messages.conversation_id, c.team_id, c.assignee_id, c.channel_type, c.channel_id;

-- name: GetThreadMessage :one
-- One message in the thread's shape, after a send or a note.
SELECT m.id, m.direction, m.kind, m.type, m.body, m.content, m.reply_to_id, m.status, m.error_code,
       m.error_title, m.author_id, m.automation_run_id, m.mentions, m.reactions, m.created_at, m.status_at,
       u.email AS author_email, p.display_name AS author_display_name,
       p.first_name AS author_first_name, p.last_name AS author_last_name,
       r.direction AS reply_direction, r.type AS reply_type, r.body AS reply_body
FROM messages m
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN user_profiles p ON p.user_id = m.author_id
LEFT JOIN messages r ON r.id = m.reply_to_id
WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id AND m.id = @id;

-- name: AutomationConversation :one
-- What automations need to know before they speak. last_agent_at is the epoch
-- when no person has written here.
SELECT c.contact_id, c.channel_type, c.window_expires_at, ct.name AS contact_name, ct.opt_in,
       (SELECT COALESCE(max(m.created_at), 'epoch')::timestamptz FROM messages m
        WHERE m.workspace_id = c.workspace_id AND m.conversation_id = c.id AND m.direction = 'out'
          AND m.kind = 'message' AND m.automation_run_id IS NULL) AS last_agent_at
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
WHERE c.workspace_id = @workspace_id AND c.id = @id;

-- name: InsertAutomatedMessage :exec
INSERT INTO messages (id, workspace_id, conversation_id, direction, kind, type, body, content, status, automation_run_id, created_at)
VALUES (@id, @workspace_id, @conversation_id, 'out', 'message', @type, @body, @content, 'pending', @automation_run_id, @created_at);

-- name: TouchAutomated :one
-- An automated reply shows in the list but answers nobody: a customer waiting
-- for a person stays waiting.
UPDATE conversations
SET last_message_at        = GREATEST(last_message_at, @at::timestamptz),
    last_message_preview   = @preview,
    last_message_direction = 'out',
    status                 = CASE WHEN @to_pending::boolean AND status = 'closed' THEN 'pending' ELSE status END,
    closed_at              = CASE WHEN @to_pending::boolean AND status = 'closed' THEN NULL ELSE closed_at END
WHERE workspace_id = @workspace_id AND id = @id
RETURNING team_id, assignee_id, channel_type, channel_id;

-- name: LatestContactConversation :one
SELECT id FROM conversations
WHERE workspace_id = @workspace_id AND contact_id = @contact_id
ORDER BY last_message_at DESC
LIMIT 1;

-- name: InsertAutomationConversation :one
-- A chat a template starts: no window until the customer writes back.
INSERT INTO conversations (id, workspace_id, contact_id, channel_type, channel_id, source, last_message_direction)
VALUES (@id, @workspace_id, @contact_id, @channel_type, @channel_id, 'automation', 'out')
ON CONFLICT (workspace_id, contact_id, channel_type, channel_id) DO UPDATE SET updated_at = now()
RETURNING id;
