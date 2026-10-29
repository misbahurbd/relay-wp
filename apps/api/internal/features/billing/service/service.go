// Package service holds billing's business rules (no HTTP, no SQL): the plan
// catalog, checkout, subscriptions, lifetime deals and their installments,
// discounts, grants (what a workspace can use) and the payment webhook inbox
// (docs/PHASE-6-BILLING.md).
//
// Phase 6 builds it in parallel tracks, one file each (see the build plan).
// Methods a track hasn't built yet return model.ErrNotImplemented. Methods
// that take a pgx.Tx run inside the caller's transaction: they are how one
// track's rules call another's.
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

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Config holds billing settings from the environment.
type Config struct {
	// AppBaseURL builds links in emails (pay an installment, billing settings).
	AppBaseURL string
	// CheckoutTTL is how long a pending checkout session lives.
	CheckoutTTL time.Duration
	// EventPayloadTTL is how long payment_events.payload is kept.
	EventPayloadTTL time.Duration
}

// eventHandler applies one normalised webhook event inside the processing
// transaction. workspaceID is already resolved from our reference.
type eventHandler func(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, ev payment.Event) error

// Service runs billing.
type Service struct {
	pool     *pgxpool.Pool
	payments *payment.Registry
	queue    Enqueuer
	cfg      Config
	log      *slog.Logger
	now      func() time.Time
	// events dispatches webhook events by type (an approved factory point:
	// a registry map filled once in New).
	events map[payment.EventType]eventHandler
}

// New builds the service. payments may be empty: then nothing can be bought.
func New(pool *pgxpool.Pool, payments *payment.Registry, queue Enqueuer, cfg Config, log *slog.Logger) *Service {
	if cfg.CheckoutTTL == 0 {
		cfg.CheckoutTTL = time.Hour
	}
	if cfg.EventPayloadTTL == 0 {
		cfg.EventPayloadTTL = 90 * 24 * time.Hour
	}
	s := &Service{
		pool: pool, payments: payments, queue: queue, cfg: cfg, log: log,
		now: func() time.Time { return time.Now().UTC() },
	}
	s.events = s.eventHandlers()
	return s
}

// gateway returns the enabled gateway for p.
func (s *Service) gateway(p payment.Provider) (payment.Gateway, error) {
	g, ok := s.payments.Get(p)
	if !ok {
		return nil, model.ErrProviderNotAvailable
	}
	return g, nil
}
