-- ---------- Mentions (docs/PHASE-7-INBOX.md AD-12) ----------

-- name: AddMentions :exec
INSERT INTO inbox_mentions (workspace_id, message_id, conversation_id, user_id)
SELECT @workspace_id, @message_id, @conversation_id, u.user_id
FROM unnest(@user_ids::uuid[]) AS u (user_id)
ON CONFLICT DO NOTHING;

-- name: ListMentions :many
-- A member's newest mentions with the note and its chat; the service drops chats
-- the member can no longer see.
SELECT im.message_id, im.conversation_id, im.seen_at, im.created_at, m.body, m.author_id,
       c.team_id, c.assignee_id, c.channel_type, c.channel_id, ct.name AS contact_name, ct.phone AS contact_phone,
       u.email AS author_email, p.display_name AS author_display_name,
       p.first_name AS author_first_name, p.last_name AS author_last_name
FROM inbox_mentions im
JOIN messages m ON m.id = im.message_id
JOIN conversations c ON c.id = im.conversation_id
JOIN contacts ct ON ct.id = c.contact_id
LEFT JOIN users u ON u.id = m.author_id
LEFT JOIN user_profiles p ON p.user_id = m.author_id
WHERE im.workspace_id = @workspace_id AND im.user_id = @user_id
ORDER BY im.created_at DESC, im.message_id DESC
LIMIT @page_size;

-- name: MarkMentionsSeen :execrows
-- Every unseen mention of the member, or only the given notes' ones.
UPDATE inbox_mentions
SET seen_at = now()
WHERE workspace_id = @workspace_id AND user_id = @user_id AND seen_at IS NULL
  AND (sqlc.narg(message_ids)::uuid[] IS NULL OR message_id = ANY(sqlc.narg(message_ids)::uuid[]));
