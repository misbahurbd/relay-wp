-- ---------- Conversations (docs/PHASE-7-INBOX.md AD-2, AD-10) ----------
-- The list queries take one slice of the member's scope at a time: one team, the
-- chats assigned to them, or everything. Team scope runs one query per team and
-- merges them in the service (ADR 0002). A NULL number_ids means every number;
-- a member's numbers restrict only WhatsApp chats (their WhatsApp channels, ADR 0010).
-- The two list queries and the count share one filter. Keep them in step.

-- name: ListConversationsNewest :many
-- Newest first. Cursor: (last_message_at, id) of the last row of the previous page.
SELECT c.id, c.contact_id, c.channel_type, c.channel_id, c.status, c.team_id, c.assignee_id, c.source,
       c.last_message_at, c.last_message_preview, c.last_message_direction, c.window_expires_at,
       c.unread_count, c.waiting_since, c.snoozed_until,
       ct.name AS contact_name, ct.phone AS contact_phone,
       u.email AS assignee_email, p.display_name AS assignee_display_name,
       p.first_name AS assignee_first_name, p.last_name AS assignee_last_name
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN users u ON u.id = c.assignee_id
LEFT JOIN user_profiles p ON p.user_id = c.assignee_id
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status))
  AND (NOT @not_closed::boolean OR c.status <> 'closed')
  AND (sqlc.narg(team_id)::uuid IS NULL OR c.team_id = sqlc.narg(team_id))
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR c.assignee_id = sqlc.narg(assignee_id))
  AND (sqlc.narg(outside_teams)::uuid[] IS NULL OR c.team_id IS NULL
       OR NOT (c.team_id = ANY(sqlc.narg(outside_teams)::uuid[])))
  AND (sqlc.narg(number_ids)::uuid[] IS NULL OR c.channel_type <> 'whatsapp'
       OR c.channel_id = ANY(sqlc.narg(number_ids)::uuid[]))
  AND (sqlc.narg(query)::text IS NULL OR c.last_message_preview ILIKE '%' || sqlc.narg(query) || '%'
       OR ct.name ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND ct.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (sqlc.narg(after_at)::timestamptz IS NULL
       OR (c.last_message_at, c.id) < (sqlc.narg(after_at)::timestamptz, @after_id::uuid))
ORDER BY c.last_message_at DESC, c.id DESC
LIMIT @page_size;

-- name: ListConversationsWaiting :many
-- Waiting longest first, then chats nobody is waiting on. Cursor: (waiting_since, id)
-- of the last row; a NULL after_at with has_cursor means the cursor is in the
-- not-waiting part.
SELECT c.id, c.contact_id, c.channel_type, c.channel_id, c.status, c.team_id, c.assignee_id, c.source,
       c.last_message_at, c.last_message_preview, c.last_message_direction, c.window_expires_at,
       c.unread_count, c.waiting_since, c.snoozed_until,
       ct.name AS contact_name, ct.phone AS contact_phone,
       u.email AS assignee_email, p.display_name AS assignee_display_name,
       p.first_name AS assignee_first_name, p.last_name AS assignee_last_name
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN users u ON u.id = c.assignee_id
LEFT JOIN user_profiles p ON p.user_id = c.assignee_id
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(status)::text IS NULL OR c.status = sqlc.narg(status))
  AND (NOT @not_closed::boolean OR c.status <> 'closed')
  AND (sqlc.narg(team_id)::uuid IS NULL OR c.team_id = sqlc.narg(team_id))
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR c.assignee_id = sqlc.narg(assignee_id))
  AND (sqlc.narg(outside_teams)::uuid[] IS NULL OR c.team_id IS NULL
       OR NOT (c.team_id = ANY(sqlc.narg(outside_teams)::uuid[])))
  AND (sqlc.narg(number_ids)::uuid[] IS NULL OR c.channel_type <> 'whatsapp'
       OR c.channel_id = ANY(sqlc.narg(number_ids)::uuid[]))
  AND (sqlc.narg(query)::text IS NULL OR c.last_message_preview ILIKE '%' || sqlc.narg(query) || '%'
       OR ct.name ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND ct.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (NOT @has_cursor::boolean
       OR (sqlc.narg(after_at)::timestamptz IS NOT NULL
           AND (c.waiting_since IS NULL OR (c.waiting_since, c.id) > (sqlc.narg(after_at)::timestamptz, @after_id::uuid)))
       OR (sqlc.narg(after_at)::timestamptz IS NULL AND c.waiting_since IS NULL AND c.id > @after_id::uuid))
ORDER BY c.waiting_since ASC NULLS LAST, c.id ASC
LIMIT @page_size;

-- name: CountConversations :one
-- The tab counts for one slice of the scope; the search doesn't narrow them.
SELECT count(c.id) FILTER (WHERE c.status = 'open') AS open_count,
       count(c.id) FILTER (WHERE c.status = 'pending') AS pending_count,
       count(c.id) FILTER (WHERE c.status = 'closed') AS closed_count,
       count(c.id) FILTER (WHERE c.status <> 'closed' AND c.assignee_id = @user_id::uuid) AS mine_count
FROM conversations c
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(team_id)::uuid IS NULL OR c.team_id = sqlc.narg(team_id))
  AND (sqlc.narg(assignee_id)::uuid IS NULL OR c.assignee_id = sqlc.narg(assignee_id))
  AND (sqlc.narg(outside_teams)::uuid[] IS NULL OR c.team_id IS NULL
       OR NOT (c.team_id = ANY(sqlc.narg(outside_teams)::uuid[])))
  AND (sqlc.narg(number_ids)::uuid[] IS NULL OR c.channel_type <> 'whatsapp'
       OR c.channel_id = ANY(sqlc.narg(number_ids)::uuid[]));

-- name: GetConversation :one
SELECT c.id, c.contact_id, c.channel_type, c.channel_id, c.status, c.team_id, c.assignee_id, c.source,
       c.last_message_at, c.last_message_preview, c.last_message_direction, c.last_inbound_at,
       c.window_expires_at, c.unread_count, c.waiting_since, c.snoozed_until, c.closed_at, c.created_at,
       ct.name AS contact_name, ct.phone AS contact_phone, ct.opt_in AS contact_opt_in,
       t.name AS team_name,
       u.email AS assignee_email, p.display_name AS assignee_display_name,
       p.first_name AS assignee_first_name, p.last_name AS assignee_last_name,
       EXISTS (SELECT 1 FROM messages m
               WHERE m.conversation_id = c.id AND m.kind = 'message' AND m.direction = 'out'
                 AND m.type = 'template' AND m.created_at > COALESCE(c.last_inbound_at, '-infinity'::timestamptz)
              )::boolean AS template_since_inbound
FROM conversations c
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN teams t ON t.id = c.team_id
LEFT JOIN users u ON u.id = c.assignee_id
LEFT JOIN user_profiles p ON p.user_id = c.assignee_id
WHERE c.workspace_id = @workspace_id AND c.id = @id;

-- name: LockConversation :one
SELECT c.id, c.contact_id, c.channel_type, c.channel_id, c.status, c.team_id, c.assignee_id, c.window_expires_at, c.unread_count
FROM conversations c
WHERE c.workspace_id = @workspace_id AND c.id = @id
FOR UPDATE;

-- name: UpsertInboundConversation :one
-- One inbound message (AD-2): creates the thread, or reopens it and moves its
-- denormalized fields. A message older than the last one (channels deliver out of
-- order) counts as unread but doesn't replace the preview. window_expires_at is
-- at + the channel's window, NULL for a channel without one. prev_window_expires_at
-- is the window before this message (the epoch for a new chat or no window), so
-- the service can say it reopened.
WITH prev AS (
    SELECT window_expires_at FROM conversations
    WHERE workspace_id = @workspace_id AND contact_id = @contact_id
      AND channel_type = @channel_type AND channel_id = @channel_id
    FOR UPDATE
)
INSERT INTO conversations (id, workspace_id, contact_id, channel_type, channel_id, status, source, last_message_at,
                           last_message_preview, last_message_direction, last_inbound_at, window_expires_at,
                           unread_count, waiting_since)
VALUES (@id, @workspace_id, @contact_id, @channel_type, @channel_id, 'open', @source, @at::timestamptz, @preview, 'in',
        @at::timestamptz, sqlc.narg(window_expires_at)::timestamptz, 1, @at::timestamptz)
ON CONFLICT (workspace_id, contact_id, channel_type, channel_id) DO UPDATE
SET status                 = 'open',
    snoozed_until          = NULL,
    closed_at              = NULL,
    last_message_preview   = CASE WHEN EXCLUDED.last_message_at >= conversations.last_message_at
                                  THEN EXCLUDED.last_message_preview ELSE conversations.last_message_preview END,
    last_message_direction = CASE WHEN EXCLUDED.last_message_at >= conversations.last_message_at
                                  THEN 'in' ELSE conversations.last_message_direction END,
    last_message_at        = GREATEST(conversations.last_message_at, EXCLUDED.last_message_at),
    last_inbound_at        = GREATEST(conversations.last_inbound_at, EXCLUDED.last_inbound_at),
    -- The same window length after the newest inbound message; NULL stays NULL.
    window_expires_at      = GREATEST(conversations.last_inbound_at, EXCLUDED.last_inbound_at)
                             + (EXCLUDED.window_expires_at - EXCLUDED.last_inbound_at),
    unread_count           = conversations.unread_count + 1,
    waiting_since          = COALESCE(conversations.waiting_since, EXCLUDED.waiting_since)
RETURNING id, team_id, assignee_id, (xmax = 0)::boolean AS created,
    COALESCE((SELECT prev.window_expires_at FROM prev), 'epoch'::timestamptz)::timestamptz AS prev_window_expires_at;

-- name: TouchOutbound :one
-- An agent's message: it answers whoever was waiting. A template to a closed chat
-- moves it to pending (AD-6).
UPDATE conversations
SET last_message_at        = GREATEST(last_message_at, @at::timestamptz),
    last_message_preview   = @preview,
    last_message_direction = 'out',
    waiting_since          = NULL,
    status                 = CASE WHEN @to_pending::boolean AND status = 'closed' THEN 'pending' ELSE status END,
    closed_at              = CASE WHEN @to_pending::boolean AND status = 'closed' THEN NULL ELSE closed_at END
WHERE workspace_id = @workspace_id AND id = @id
RETURNING status, team_id, assignee_id, channel_type, channel_id;

-- name: SetAssignee :one
UPDATE conversations
SET assignee_id = sqlc.narg(assignee_id)
WHERE workspace_id = @workspace_id AND id = @id
RETURNING team_id, assignee_id, channel_type, channel_id;

-- name: SetConversationStatus :one
UPDATE conversations
SET status        = @status,
    snoozed_until = sqlc.narg(snoozed_until),
    closed_at     = CASE WHEN @status = 'closed' THEN now() ELSE NULL END
WHERE workspace_id = @workspace_id AND id = @id
RETURNING team_id, assignee_id, channel_type, channel_id;

-- name: ClearUnread :one
UPDATE conversations
SET unread_count = 0
WHERE workspace_id = @workspace_id AND id = @id
RETURNING team_id, assignee_id, channel_type, channel_id;

-- name: ReopenSnoozed :many
-- Chats whose snooze has ended, a batch at a time, across workspaces.
UPDATE conversations
SET status = 'open', snoozed_until = NULL
WHERE id IN (SELECT s.id FROM conversations s
             WHERE s.snoozed_until <= @now::timestamptz
             ORDER BY s.snoozed_until
             LIMIT @batch_size
             FOR UPDATE SKIP LOCKED)
RETURNING id, workspace_id, team_id, assignee_id, channel_type, channel_id;
