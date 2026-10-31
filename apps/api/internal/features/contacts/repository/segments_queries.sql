-- ---------- Segments (ADR 0014) ----------
-- Who matches a segment is compiled from its rules in segments_match.go.

-- name: ListSegments :many
SELECT id, name, description, rules, created_at, updated_at
FROM contact_segments
WHERE workspace_id = @workspace_id
ORDER BY name
LIMIT 200;

-- name: GetSegment :one
SELECT id, name, description, rules, created_at, updated_at
FROM contact_segments
WHERE workspace_id = @workspace_id AND id = @id;

-- name: InsertSegment :exec
INSERT INTO contact_segments (id, workspace_id, name, description, rules, created_by)
VALUES (@id, @workspace_id, @name, @description, @rules, sqlc.narg('created_by'));

-- name: UpdateSegment :execrows
UPDATE contact_segments SET name = @name, description = @description, rules = @rules, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id;

-- name: DeleteSegment :execrows
DELETE FROM contact_segments WHERE workspace_id = @workspace_id AND id = @id;

-- name: SegmentRules :many
-- The rules of the given segments, for resolving a broadcast's audience.
SELECT id, rules FROM contact_segments WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]);
