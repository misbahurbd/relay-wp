-- name: CreateWorkspace :exec
INSERT INTO workspaces (id, name, trial_ends_at, created_by)
VALUES (@id, @name, @trial_ends_at, @created_by);

-- name: CountTrialWorkspacesOwned :one
-- Workspaces this user owns that haven't sent a real message yet.
SELECT count(*)
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
JOIN workspaces w ON w.id = m.workspace_id
WHERE m.user_id = @user_id AND r.system_key = 'owner' AND w.activated_at IS NULL;

-- name: ListMemberWorkspaces :many
SELECT w.id, w.name, r.name AS role_name, COALESCE(r.system_key = 'owner', false)::boolean AS is_owner,
       m.status, m.last_active_at, w.activated_at
FROM workspace_members m
JOIN roles r ON r.id = m.role_id
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
JOIN roles r ON r.workspace_id = w.id AND r.system_key = 'owner'
JOIN workspace_members m ON m.workspace_id = w.id AND m.role_id = r.id
WHERE w.id = @workspace_id
ORDER BY m.created_at
LIMIT 1;

-- name: UpdateWorkspaceBusiness :exec
UPDATE workspaces
SET name = @name, industry = @industry, website = @website, country = @country,
    team_size = @team_size, goals = @goals
WHERE id = @workspace_id;

-- name: LockSeatLimit :one
-- Locks the workspace row so seat checks in concurrent invites and accepts run one at a time.
SELECT seat_limit
FROM workspaces
WHERE id = @workspace_id
FOR UPDATE;

-- name: TouchWorkspaceMember :exec
UPDATE workspace_members
SET last_active_at = now()
WHERE workspace_id = @workspace_id AND user_id = @user_id;

-- name: MarkWorkspaceActivated :execrows
UPDATE workspaces
SET activated_at = now()
WHERE id = @workspace_id AND activated_at IS NULL;
