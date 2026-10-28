-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, name, trial_ends_at, created_by)
VALUES (@id, @name, @trial_ends_at, @created_by);

-- name: AddWorkspaceMember :exec
INSERT INTO workspace_members (workspace_id, user_id, role)
VALUES (@workspace_id, @user_id, @role);

-- name: CountTrialWorkspacesOwned :one
-- Workspaces this user owns that haven't sent a real message yet.
SELECT count(*)
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = @user_id AND m.role = 'owner' AND w.activated_at IS NULL;

-- name: ListMemberWorkspaces :many
SELECT w.id, w.name, m.role, m.last_active_at, w.activated_at
FROM workspace_members m
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = @user_id
ORDER BY m.last_active_at DESC, w.created_at DESC;

-- name: GetWorkspace :one
SELECT id, name, industry, website, country, team_size, goals, plan, trial_ends_at, activated_at
FROM workspaces
WHERE id = @workspace_id;

-- name: GetWorkspaceOwner :one
SELECT w.name, m.user_id
FROM workspaces w
JOIN workspace_members m ON m.workspace_id = w.id AND m.role = 'owner'
WHERE w.id = @workspace_id
ORDER BY m.created_at
LIMIT 1;

-- name: UpdateWorkspaceBusiness :exec
UPDATE workspaces
SET name = @name, industry = @industry, website = @website, country = @country,
    team_size = @team_size, goals = @goals
WHERE id = @workspace_id;

-- name: GetMemberRole :one
SELECT role
FROM workspace_members
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: TouchWorkspaceMember :exec
UPDATE workspace_members
SET last_active_at = now()
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: MarkWorkspaceActivated :execrows
UPDATE workspaces
SET activated_at = now()
WHERE id = @workspace_id AND activated_at IS NULL;
