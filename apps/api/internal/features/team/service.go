// Package team owns who can do what in a workspace: roles, members and their
// per-person permissions, teams, invitations and the access log
// (docs/PHASE-3-TEAM-ACCESS.md).
package team

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Workspaces reads the plan's seat limit. Implemented by workspaces.Directory.
type Workspaces interface {
	// LockSeatLimit locks the workspace row (in db's transaction) so seat checks run one at a time.
	LockSeatLimit(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) (int32, error)
}

// AccountReader returns a user's email and whether it's verified. Implemented by auth.Accounts.
type AccountReader interface {
	AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Config holds team settings from the environment.
type Config struct {
	// AppBaseURL builds invite links.
	AppBaseURL string
}

// Service manages roles, members, teams and invitations.
type Service struct {
	pool       *pgxpool.Pool
	workspaces Workspaces
	accounts   AccountReader
	queue      Enqueuer
	cfg        Config
	now        func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, workspaces Workspaces, accounts AccountReader, queue Enqueuer, cfg Config) *Service {
	return &Service{
		pool: pool, workspaces: workspaces, accounts: accounts, queue: queue, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Seed gives a new workspace its four system roles and makes ownerID its Owner.
// workspaces.Service calls it inside the create-workspace transaction.
func (s *Service) Seed(ctx context.Context, tx pgx.Tx, workspaceID, ownerID uuid.UUID) error {
	r := newRepository(tx)
	var ownerRole uuid.UUID
	for _, sr := range systemRoles {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("role id: %w", err)
		}
		key := sr.key
		if err := r.CreateRole(ctx, gen.CreateRoleParams{
			ID: id, WorkspaceID: workspaceID, SystemKey: &key, Name: sr.name, Description: sr.description,
			Color: sr.color, ChatScope: sr.scope, UsesSeat: sr.key != RoleViewer, Locked: sr.key == RoleOwner,
		}); err != nil {
			return fmt.Errorf("create %s role: %w", sr.key, err)
		}
		if err := r.SetRolePermissions(ctx, gen.SetRolePermissionsParams{
			WorkspaceID: workspaceID, RoleID: id, Keys: keysOf(sr.perms),
		}); err != nil {
			return fmt.Errorf("%s role permissions: %w", sr.key, err)
		}
		if sr.key == RoleOwner {
			ownerRole = id
		}
	}
	if err := r.AddMember(ctx, gen.AddMemberParams{WorkspaceID: workspaceID, UserID: ownerID, RoleID: ownerRole}); err != nil {
		return fmt.Errorf("add owner: %w", err)
	}
	return nil
}
