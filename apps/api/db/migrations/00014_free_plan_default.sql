-- +goose Up
-- Billing owns workspaces.plan (a cache of the current grant, Q8). With no
-- grant a workspace is on the free plan, and only a trial grant sets
-- trial_ends_at. Workspaces made before billing get the same.
ALTER TABLE workspaces ALTER COLUMN plan SET DEFAULT 'free';
UPDATE workspaces w
SET plan = 'free', trial_ends_at = NULL
WHERE NOT EXISTS (
    SELECT 1 FROM plan_grants g WHERE g.workspace_id = w.id AND g.status IN ('active', 'suspended')
);

-- +goose Down
ALTER TABLE workspaces ALTER COLUMN plan SET DEFAULT 'growth';
