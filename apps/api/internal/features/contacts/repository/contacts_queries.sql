-- ---------- Contacts ----------
-- The list, count, export and attribute-key queries share one filter. Keep them in step.

-- name: ListContacts :many
-- Cursor: id of the last row of the previous page (ids are UUIDv7, so id DESC is newest first).
SELECT c.id, c.name, c.phone, c.email, c.opt_in, c.source, c.language, c.assigned_to,
       c.last_message_at, c.created_at, c.updated_at,
       u.email AS assigned_email, p.display_name AS assigned_display_name,
       p.first_name AS assigned_first_name, p.last_name AS assigned_last_name
FROM contacts c
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN user_profiles p ON p.user_id = c.assigned_to
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(opt_in)::text IS NULL OR c.opt_in = sqlc.narg(opt_in))
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM contact_tag_links l WHERE l.contact_id = c.id AND l.tag_id = sqlc.narg(tag_id)))
  AND (sqlc.narg(query)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(query) || '%'
       OR c.email::text ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND c.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (sqlc.narg(message_after)::timestamptz IS NULL OR c.last_message_at >= sqlc.narg(message_after))
  AND (sqlc.narg(message_before)::timestamptz IS NULL OR c.last_message_at < sqlc.narg(message_before))
  AND (sqlc.narg(after_id)::uuid IS NULL OR c.id < sqlc.narg(after_id))
ORDER BY c.id DESC
LIMIT @page_size;

-- name: CountContacts :one
SELECT count(c.id)
FROM contacts c
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(opt_in)::text IS NULL OR c.opt_in = sqlc.narg(opt_in))
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM contact_tag_links l WHERE l.contact_id = c.id AND l.tag_id = sqlc.narg(tag_id)))
  AND (sqlc.narg(query)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(query) || '%'
       OR c.email::text ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND c.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (sqlc.narg(message_after)::timestamptz IS NULL OR c.last_message_at >= sqlc.narg(message_after))
  AND (sqlc.narg(message_before)::timestamptz IS NULL OR c.last_message_at < sqlc.narg(message_before));

-- name: ExportContacts :many
-- One page of the export, with tag names joined and the custom fields.
SELECT c.id, c.name, c.phone, c.email, c.opt_in, c.source, c.language, c.attributes, c.created_at,
       COALESCE((SELECT string_agg(t.name::text, ', ' ORDER BY t.name)
                 FROM contact_tag_links l JOIN contact_tags t ON t.id = l.tag_id
                 WHERE l.contact_id = c.id), '')::text AS tags
FROM contacts c
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(opt_in)::text IS NULL OR c.opt_in = sqlc.narg(opt_in))
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM contact_tag_links l WHERE l.contact_id = c.id AND l.tag_id = sqlc.narg(tag_id)))
  AND (sqlc.narg(query)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(query) || '%'
       OR c.email::text ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND c.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (sqlc.narg(message_after)::timestamptz IS NULL OR c.last_message_at >= sqlc.narg(message_after))
  AND (sqlc.narg(message_before)::timestamptz IS NULL OR c.last_message_at < sqlc.narg(message_before))
  AND (sqlc.narg(after_id)::uuid IS NULL OR c.id < sqlc.narg(after_id))
ORDER BY c.id DESC
LIMIT @page_size;

-- name: ContactAttributeKeys :many
-- Every custom-field key among the matching contacts, for the export's columns.
SELECT DISTINCT k.key::text AS key
FROM contacts c, jsonb_object_keys(c.attributes) AS k(key)
WHERE c.workspace_id = @workspace_id
  AND (sqlc.narg(opt_in)::text IS NULL OR c.opt_in = sqlc.narg(opt_in))
  AND (sqlc.narg(tag_id)::uuid IS NULL OR EXISTS (
        SELECT 1 FROM contact_tag_links l WHERE l.contact_id = c.id AND l.tag_id = sqlc.narg(tag_id)))
  AND (sqlc.narg(query)::text IS NULL OR c.name ILIKE '%' || sqlc.narg(query) || '%'
       OR c.email::text ILIKE '%' || sqlc.narg(query) || '%'
       OR (sqlc.narg(phone_query)::text IS NOT NULL AND c.phone LIKE '%' || sqlc.narg(phone_query) || '%'))
  AND (sqlc.narg(message_after)::timestamptz IS NULL OR c.last_message_at >= sqlc.narg(message_after))
  AND (sqlc.narg(message_before)::timestamptz IS NULL OR c.last_message_at < sqlc.narg(message_before))
ORDER BY 1;

-- name: ContactSummary :one
SELECT count(c.id) AS total, count(c.id) FILTER (WHERE c.opt_in = 'in') AS opted_in
FROM contacts c
WHERE c.workspace_id = @workspace_id;

-- name: ListTagsOfContacts :many
SELECT l.contact_id, t.id, t.name
FROM contact_tag_links l
JOIN contact_tags t ON t.id = l.tag_id
WHERE l.workspace_id = @workspace_id AND l.contact_id = ANY(@contact_ids::uuid[])
ORDER BY t.name;

-- name: GetContact :one
SELECT c.id, c.name, c.phone, c.email, c.opt_in, c.opt_in_source, c.opt_in_at, c.source, c.language,
       c.attributes, c.assigned_to, c.last_message_at, c.created_at, c.updated_at,
       u.email AS assigned_email, p.display_name AS assigned_display_name,
       p.first_name AS assigned_first_name, p.last_name AS assigned_last_name
FROM contacts c
LEFT JOIN users u ON u.id = c.assigned_to
LEFT JOIN user_profiles p ON p.user_id = c.assigned_to
WHERE c.workspace_id = @workspace_id AND c.id = @id;

-- name: LockContact :one
SELECT c.name, c.phone, c.email, c.opt_in, c.opt_in_source, c.opt_in_at, c.language, c.attributes
FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.id = @id
FOR UPDATE;

-- name: CreateContact :exec
INSERT INTO contacts (id, workspace_id, name, phone, email, opt_in, opt_in_source, opt_in_at, source, language, attributes)
VALUES (@id, @workspace_id, @name, @phone, @email, @opt_in, @opt_in_source, @opt_in_at, @source, @language, @attributes);

-- name: UpdateContact :exec
UPDATE contacts
SET name = @name, phone = @phone, email = @email, opt_in = @opt_in, opt_in_source = @opt_in_source,
    opt_in_at = @opt_in_at, language = @language, attributes = @attributes
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteContacts :execrows
DELETE FROM contacts
WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: AssignContacts :execrows
UPDATE contacts
SET assigned_to = sqlc.narg(user_id)
WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);

-- name: AddContactTagLinks :execrows
-- Pairs (contact_ids[i], tag_ids[i]); pairs outside the workspace are dropped.
INSERT INTO contact_tag_links (workspace_id, contact_id, tag_id)
SELECT @workspace_id, c.id, t.id
FROM (SELECT unnest(@contact_ids::uuid[]) AS contact_id, unnest(@tag_ids::uuid[]) AS tag_id) AS u
JOIN contacts c ON c.id = u.contact_id AND c.workspace_id = @workspace_id
JOIN contact_tags t ON t.id = u.tag_id AND t.workspace_id = @workspace_id
ON CONFLICT DO NOTHING;

-- name: RemoveContactTagLinks :execrows
DELETE FROM contact_tag_links
WHERE workspace_id = @workspace_id AND tag_id = @tag_id AND contact_id = ANY(@contact_ids::uuid[]);

-- name: ClearContactTags :exec
DELETE FROM contact_tag_links
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: ExistingContactPhones :many
SELECT c.phone, c.opt_in
FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.phone = ANY(@phones::text[]);

-- name: UpsertImportedContacts :many
-- One import batch. A new contact is opted in from the import; an existing one keeps its values
-- except the non-empty imported ones, and an opt-out stays out (an import never re-subscribes).
INSERT INTO contacts (id, workspace_id, name, phone, email, opt_in, opt_in_source, opt_in_at, source, attributes)
SELECT u.id, @workspace_id, u.name, u.phone, NULLIF(u.email, '')::citext, 'in', 'import', now(), 'import',
       u.attributes::jsonb
FROM (SELECT unnest(@ids::uuid[]) AS id, unnest(@names::text[]) AS name, unnest(@phones::text[]) AS phone,
             unnest(@emails::text[]) AS email, unnest(@attributes::text[]) AS attributes) AS u
ON CONFLICT (workspace_id, phone) DO UPDATE
SET name          = CASE WHEN EXCLUDED.name <> '' THEN EXCLUDED.name ELSE contacts.name END,
    email         = COALESCE(EXCLUDED.email, contacts.email),
    attributes    = contacts.attributes || EXCLUDED.attributes,
    opt_in        = CASE WHEN contacts.opt_in = 'pending' THEN 'in' ELSE contacts.opt_in END,
    opt_in_source = CASE WHEN contacts.opt_in = 'pending' THEN 'import' ELSE contacts.opt_in_source END,
    opt_in_at     = CASE WHEN contacts.opt_in = 'pending' THEN now() ELSE contacts.opt_in_at END
RETURNING id, phone, (xmax = 0)::boolean AS inserted;

-- name: LockContactCap :exec
-- Serializes adding contacts in one workspace, so two adds can't both pass the plan's cap.
SELECT pg_advisory_xact_lock(hashtextextended('contacts:' || sqlc.arg(workspace_id)::uuid::text, 0));

-- name: CountWorkspaceContacts :one
SELECT count(*) FROM contacts WHERE workspace_id = @workspace_id;

-- ---------- Inbox (docs/PHASE-7-INBOX.md F1) ----------

-- name: InsertInboxContact :many
-- A customer who wrote in. Nothing happens when the phone is already a contact,
-- so concurrent webhooks create it once; the row comes back only when inserted.
INSERT INTO contacts (id, workspace_id, name, phone, source)
VALUES (@id, @workspace_id, @name, @phone, 'inbox')
ON CONFLICT (workspace_id, phone) DO NOTHING
RETURNING id;

-- name: ContactIDByPhone :one
SELECT c.id
FROM contacts c
WHERE c.workspace_id = @workspace_id AND c.phone = @phone;

-- name: SetContactLastMessage :exec
UPDATE contacts
SET last_message_at = GREATEST(last_message_at, @at::timestamptz)
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ContactIDByIdentity :one
-- The contact a channel's sender is (ADR 0010).
SELECT ci.contact_id
FROM contact_identities ci
WHERE ci.workspace_id = @workspace_id AND ci.channel_type = @channel_type AND ci.external_id = @external_id;

-- name: InsertContactIdentity :exec
-- Links a sender to a contact. A concurrent link of the same sender wins; the
-- caller reads the identity back.
INSERT INTO contact_identities (workspace_id, contact_id, channel_type, external_id, handle)
VALUES (@workspace_id, @contact_id, @channel_type, @external_id, @handle)
ON CONFLICT (workspace_id, channel_type, external_id) DO NOTHING;

-- name: SetContactOptOut :execrows
UPDATE contacts
SET opt_in = 'out', opt_in_source = @source, opt_in_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND opt_in <> 'out';

-- name: ContactOptIn :one
SELECT opt_in FROM contacts WHERE workspace_id = @workspace_id AND id = @id;
