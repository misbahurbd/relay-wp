// Package service holds contacts' business rules (no HTTP, no SQL): contacts identified by their
// E.164 phone, tags, bulk actions, the CSV export and the import flow
// (docs/PHASE-4-CONTACTS-TEMPLATES.md AD-2 to AD-5).
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/storage"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Members tells whether someone belongs to a workspace, for assigning contacts.
// Implemented by team.Resolver.
type Members interface {
	ResolveAccess(ctx context.Context, workspaceID, userID uuid.UUID) (a authctx.Access, suspended, ok bool, err error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Service manages contacts, their tags and imports.
type Service struct {
	pool    *pgxpool.Pool
	store   storage.Storage
	queue   Enqueuer
	members Members
	now     func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, store storage.Storage, queue Enqueuer, members Members) *Service {
	return &Service{
		pool: pool, store: store, queue: queue, members: members,
		now: func() time.Time { return time.Now().UTC() },
	}
}
