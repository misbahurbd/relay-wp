// Package service holds broadcasts' business rules (no HTTP, no SQL): drafts,
// the audience, sending in batches and reports (ADR 0013).
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
)

// Inbox queues one broadcast message. Implemented in internal/app on top of features/inbox.
type Inbox interface {
	// SendBroadcast queues the message in tx and returns its ID. When it can't go
	// it writes nothing and returns model.ErrSendOptedOut, ErrSendTemplate,
	// ErrSendMissingVariable or ErrSendNoChannel.
	SendBroadcast(ctx context.Context, tx pgx.Tx, in model.Send) (uuid.UUID, error)
}

// Templates reads a template. Implemented in internal/app on top of features/templates.
type Templates interface {
	// ApprovedTemplate returns model.ErrTemplateNotApproved when the template is gone or not approved.
	ApprovedTemplate(ctx context.Context, workspaceID, templateID uuid.UUID) (model.Template, error)
}

// Numbers reads a WhatsApp number. Implemented in internal/app on top of features/whatsapp.
type Numbers interface {
	// Number returns model.ErrNumberNotConnected when the number isn't the workspace's.
	Number(ctx context.Context, workspaceID, numberID uuid.UUID) (model.Number, error)
}

// Segments resolves contact segments to their members. Implemented by features/contacts (ADR 0014).
type Segments interface {
	// Members is every contact in any of the segments, once; model.ErrSegmentNotFound when one is gone.
	Members(ctx context.Context, workspaceID uuid.UUID, segmentIDs []uuid.UUID) ([]uuid.UUID, error)
}

// Usage counts the plan's monthly broadcasts. Implemented by features/billing (ADR 0008).
type Usage interface {
	Consume(ctx context.Context, workspaceID uuid.UUID, key string, n int) (period time.Time, ok bool, err error)
	Release(ctx context.Context, workspaceID uuid.UUID, key string, period time.Time, n int) error
}

// Events publishes integrations webhooks in tx. Implemented by features/integrations.
type Events interface {
	Publish(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, eventType string, data any) error
}

// Activation marks a workspace activated once it reached a real customer.
// Implemented by features/workspaces (docs/PHASE-3-ONBOARDING.md).
type Activation interface {
	MarkActivated(ctx context.Context, workspaceID uuid.UUID) (bool, error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Deps are the service's collaborators, wired in internal/app.
type Deps struct {
	Inbox      Inbox
	Templates  Templates
	Numbers    Numbers
	Segments   Segments
	Usage      Usage
	Events     Events
	Activation Activation
}

// Service manages a workspace's broadcasts and sends them.
type Service struct {
	pool  *pgxpool.Pool
	queue Enqueuer
	deps  Deps
	log   *slog.Logger
	now   func() time.Time
}

// New builds the service. Call Connect before sending.
func New(pool *pgxpool.Pool, queue Enqueuer, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{pool: pool, queue: queue, log: log, now: func() time.Time { return time.Now().UTC() }}
}

// Connect passes in the features that are built after this one.
func (s *Service) Connect(deps Deps) { s.deps = deps }
