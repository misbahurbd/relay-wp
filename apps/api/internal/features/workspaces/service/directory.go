package service

import (
	"context"
	"fmt"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Directory gives the membership middleware and other features read access to
// workspaces. It has no dependency on Service, so it can be built first.
type Directory struct {
	pool *pgxpool.Pool
}

// NewDirectory returns a Directory.
func NewDirectory(pool *pgxpool.Pool) *Directory { return &Directory{pool: pool} }

// LockSeatLimit locks the workspace row in db's transaction and returns its seat limit,
// so seat checks in features/team run one at a time.
func (d *Directory) LockSeatLimit(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) (int32, error) {
	limit, err := repository.NewRepository(db).LockSeatLimit(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return 0, model.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lock seat limit: %w", err)
	}
	return limit, nil
}

// Owner returns the workspace's name and its owner, for reminder emails.
func (d *Directory) Owner(ctx context.Context, workspaceID uuid.UUID) (name string, ownerID uuid.UUID, err error) {
	row, err := repository.NewRepository(d.pool).GetWorkspaceOwner(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return "", uuid.Nil, model.ErrNotFound
	}
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("workspace owner: %w", err)
	}
	return row.Name, row.UserID, nil
}

// Trial returns when the trial ends and whether the workspace is activated.
func (d *Directory) Trial(ctx context.Context, workspaceID uuid.UUID) (endsAt *time.Time, activated bool, err error) {
	row, err := repository.NewRepository(d.pool).GetWorkspace(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return nil, false, model.ErrNotFound
	}
	if err != nil {
		return nil, false, fmt.Errorf("get workspace: %w", err)
	}
	return row.TrialEndsAt, row.ActivatedAt != nil, nil
}

// MarkActivated records the first real customer message. Only the first call
// sets it; it reports whether this call did. Broadcasts and Inbox will call it.
func (d *Directory) MarkActivated(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	n, err := repository.NewRepository(d.pool).MarkWorkspaceActivated(ctx, workspaceID)
	if err != nil {
		return false, fmt.Errorf("mark activated: %w", err)
	}
	return n == 1, nil
}
