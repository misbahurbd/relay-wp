// Package service holds automations' business rules (no HTTP, no SQL).
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// Inbox reads conversations and sends automated messages. Implemented in
// internal/app on top of features/inbox.
type Inbox interface {
	Conversation(ctx context.Context, workspaceID, conversationID uuid.UUID) (model.Conversation, error)
	// SendAutomated queues a message in tx. When it can't go it writes nothing and
	// returns model.ErrWindowClosed, ErrOptedOut, ErrTemplateUnavailable,
	// ErrMissingVariable or ErrNoChannel.
	SendAutomated(ctx context.Context, tx pgx.Tx, in model.AutomatedSend) error
	// NoteOptOut adds a line to the thread saying the customer opted out.
	NoteOptOut(ctx context.Context, workspaceID, conversationID uuid.UUID) error
}

// Contacts changes a contact's consent. Implemented by features/contacts.
type Contacts interface {
	// SetOptOut opts the contact out; changed is false if they already were.
	SetOptOut(ctx context.Context, workspaceID, contactID uuid.UUID, source string) (changed bool, err error)
	// FindOrCreateByPhone returns model.ErrInvalidPhone for a phone nobody can have.
	FindOrCreateByPhone(ctx context.Context, workspaceID uuid.UUID, phone, name string) (uuid.UUID, error)
	// ContactIDByPhone says who has a phone, if anyone.
	ContactIDByPhone(ctx context.Context, workspaceID uuid.UUID, phone string) (uuid.UUID, bool, error)
	// Contact reads a contact's consent and name.
	Contact(ctx context.Context, workspaceID, contactID uuid.UUID) (model.Contact, error)
}

// PlanAccess says whether a plan includes a resource. Implemented by features/billing.
type PlanAccess interface {
	HasAccess(ctx context.Context, workspaceID uuid.UUID, key string) (bool, error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Deps are the service's collaborators, wired in internal/app.
type Deps struct {
	Inbox    Inbox
	Contacts Contacts
	Plans    PlanAccess
}

// Service manages a workspace's automations and runs them.
type Service struct {
	pool  *pgxpool.Pool
	queue Enqueuer
	deps  Deps
	now   func() time.Time
}

// New builds the service. Call Connect before running automations.
func New(pool *pgxpool.Pool, queue Enqueuer) *Service {
	return &Service{pool: pool, queue: queue, now: func() time.Time { return time.Now().UTC() }}
}

// Connect passes in the features that are built after this one.
func (s *Service) Connect(deps Deps) { s.deps = deps }

// List returns all six kinds in page order. A kind nobody saved is off, with
// its defaults.
func (s *Service) List(ctx context.Context, workspaceID uuid.UUID) ([]model.Automation, error) {
	rows, err := repository.NewRepository(s.pool).ListAutomations(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list automations: %w", err)
	}
	stats, err := s.thisMonth(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	saved := make(map[model.Kind]gen.ListAutomationsRow, len(rows))
	for _, r := range rows {
		saved[model.Kind(r.Kind)] = r
	}
	out := make([]model.Automation, len(model.Kinds))
	for i, k := range model.Kinds {
		r, ok := saved[k]
		if !ok {
			out[i] = model.Automation{Kind: k, Settings: model.Default(k)}
			continue
		}
		if out[i], err = toAutomation(r.Kind, r.Enabled, r.Settings, r.UpdatedAt); err != nil {
			return nil, err
		}
		out[i].Stats = stats[k]
	}
	return out, nil
}

func (s *Service) thisMonth(ctx context.Context, workspaceID uuid.UUID) (map[model.Kind]model.Stats, error) {
	now := s.now()
	repo := repository.NewRepository(s.pool)
	rows, err := repo.RunStats(ctx, gen.RunStatsParams{
		WorkspaceID: workspaceID, Since: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		return nil, fmt.Errorf("automation stats: %w", err)
	}
	out := make(map[model.Kind]model.Stats, len(rows))
	active, err := repo.ActiveRuns(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("active runs: %w", err)
	}
	defer func() {
		for _, a := range active {
			st := out[model.Kind(a.Kind)]
			st.Active = a.Active
			out[model.Kind(a.Kind)] = st
		}
	}()
	for _, r := range rows {
		out[model.Kind(r.Kind)] = model.Stats{
			Sent: r.Sent, Failed: r.Failed, Read: r.Read,
			Recovered: r.Recovered, RecoveredMinor: r.RecoveredMinor, RecoveredCurrency: r.RecoveredCurrency,
		}
	}
	return out, nil
}

// Save replaces a kind's settings.
func (s *Service) Save(ctx context.Context, workspaceID, userID uuid.UUID, kind model.Kind, in model.Settings) (model.Automation, error) {
	if d := in.Validate(); len(d) > 0 {
		return model.Automation{}, model.ErrInvalidSettings.WithDetails(d...)
	}
	id, raw, err := newRow(in)
	if err != nil {
		return model.Automation{}, err
	}
	r, err := repository.NewRepository(s.pool).SaveAutomationSettings(ctx, gen.SaveAutomationSettingsParams{
		ID: id, WorkspaceID: workspaceID, Kind: string(kind), Settings: raw, UpdatedBy: &userID,
	})
	if err != nil {
		return model.Automation{}, fmt.Errorf("save automation: %w", err)
	}
	return toAutomation(r.Kind, r.Enabled, r.Settings, r.UpdatedAt)
}

// SetEnabled turns a kind on or off. Turning on needs its settings complete, and
// resuming a drip moves its dates on by the time it was paused.
func (s *Service) SetEnabled(ctx context.Context, workspaceID, userID uuid.UUID, kind model.Kind, enabled bool) (model.Automation, error) {
	var out model.Automation
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		current := model.Default(kind)
		var pausedAt *time.Time
		row, err := repo.GetAutomation(ctx, gen.GetAutomationParams{WorkspaceID: workspaceID, Kind: string(kind)})
		switch {
		case err == nil:
			pausedAt = row.PausedAt
			if current, err = model.Decode(kind, row.Settings); err != nil {
				return err
			}
		case !repository.IsNoRows(err):
			return fmt.Errorf("get automation: %w", err)
		}
		if d := current.Ready(); enabled && len(d) > 0 {
			return model.ErrNotReady.WithDetails(d...)
		}
		id, raw, err := newRow(current)
		if err != nil {
			return err
		}
		r, err := repo.SetAutomationEnabled(ctx, gen.SetAutomationEnabledParams{
			ID: id, WorkspaceID: workspaceID, Kind: string(kind), Enabled: enabled, Settings: raw, UpdatedBy: &userID,
		})
		if err != nil {
			return fmt.Errorf("set automation enabled: %w", err)
		}
		if kind == model.KindDrip && enabled && pausedAt != nil {
			if err := repo.ShiftDripRuns(ctx, gen.ShiftDripRunsParams{WorkspaceID: workspaceID, PausedSeconds: s.now().Sub(*pausedAt).Seconds()}); err != nil {
				return fmt.Errorf("shift drip runs: %w", err)
			}
		}
		out, err = toAutomation(r.Kind, r.Enabled, r.Settings, r.UpdatedAt)
		return err
	})
	return out, err
}

func newRow(settings model.Settings) (uuid.UUID, []byte, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("new automation id: %w", err)
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return uuid.Nil, nil, fmt.Errorf("encode automation settings: %w", err)
	}
	return id, raw, nil
}

func toAutomation(kind string, enabled bool, raw []byte, updatedAt time.Time) (model.Automation, error) {
	k := model.Kind(kind)
	settings, err := model.Decode(k, raw)
	if err != nil {
		return model.Automation{}, err
	}
	return model.Automation{Kind: k, Enabled: enabled, Settings: settings, UpdatedAt: &updatedAt}, nil
}
