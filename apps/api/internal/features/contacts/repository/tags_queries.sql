-- ---------- Tags ----------

-- name: ListContactTags :many
SELECT t.id, t.name::text AS name,
       (SELECT count(l.contact_id) FROM contact_tag_links l WHERE l.tag_id = t.id)::bigint AS contact_count
FROM contact_tags t
WHERE t.workspace_id = @workspace_id
ORDER BY t.name;

-- name: UpsertContactTag :one
-- Returns the tag with this name (case-insensitive), creating it when there is none.
INSERT INTO contact_tags (id, workspace_id, name)
VALUES (@id, @workspace_id, @name)
ON CONFLICT (workspace_id, name) DO UPDATE SET name = contact_tags.name
RETURNING id, name::text AS name, (xmax = 0)::boolean AS created;

-- name: GetContactTag :one
SELECT t.id, t.name::text AS name
FROM contact_tags t
WHERE t.workspace_id = @workspace_id AND t.id = @id;

-- name: CountContactTagsIn :one
-- How many of ids are tags of this workspace.
SELECT count(t.id)
FROM contact_tags t
WHERE t.workspace_id = @workspace_id AND t.id = ANY(@ids::uuid[]);
