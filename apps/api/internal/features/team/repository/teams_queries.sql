-- ---------- Teams ----------

-- name: ListTeams :many
SELECT t.id, t.name, t.description, t.color, t.number_id, t.routing_method,
       ARRAY(SELECT tm.user_id FROM team_members tm WHERE tm.team_id = t.id ORDER BY tm.created_at)::uuid[] AS member_ids
FROM teams t
WHERE t.workspace_id = @workspace_id
ORDER BY lower(t.name);

-- name: GetTeam :one
SELECT t.id, t.name, t.description, t.color, t.number_id, t.routing_method,
       ARRAY(SELECT tm.user_id FROM team_members tm WHERE tm.team_id = t.id ORDER BY tm.created_at)::uuid[] AS member_ids
FROM teams t
WHERE t.workspace_id = @workspace_id AND t.id = @team_id;

-- name: CreateTeam :exec
INSERT INTO teams (id, workspace_id, name, description, color, number_id, routing_method)
VALUES (@id, @workspace_id, @name, @description, @color, @number_id, @routing_method);

-- name: UpdateTeam :execrows
UPDATE teams
SET name = @name, description = @description, color = @color, number_id = @number_id, routing_method = @routing_method
WHERE workspace_id = @workspace_id AND id = @team_id;

-- name: DeleteTeam :execrows
DELETE FROM teams
WHERE workspace_id = @workspace_id AND id = @team_id;

-- name: TeamMemberIDs :many
SELECT user_id
FROM team_members
WHERE workspace_id = @workspace_id AND team_id = @team_id;

-- name: SetTeamMembers :exec
WITH removed AS (
    DELETE FROM team_members
    WHERE workspace_id = @workspace_id AND team_id = @team_id AND NOT (user_id = ANY (@user_ids::uuid[]))
)
INSERT INTO team_members (workspace_id, team_id, user_id)
SELECT @workspace_id, @team_id, u FROM unnest(@user_ids::uuid[]) AS u
ON CONFLICT DO NOTHING;
