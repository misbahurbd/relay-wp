// Package service holds onboarding's business rules (no HTTP, no SQL). The onboarding feature keeps
// a workspace's setup progress, the dashboard checklist, each member's tour state, and setup and
// trial reminders.
package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/onboarding/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
)

// Workspaces reads workspace fields. Implemented by workspaces.Directory.
type Workspaces interface {
	Owner(ctx context.Context, workspaceID uuid.UUID) (name string, ownerID uuid.UUID, err error)
	Trial(ctx context.Context, workspaceID uuid.UUID) (endsAt *time.Time, activated bool, err error)
}

// Channels says whether a workspace has a working WhatsApp number. Implemented by whatsapp.Channels.
type Channels interface {
	HasConnectedNumber(ctx context.Context, workspaceID uuid.UUID) (bool, error)
}

// AccountReader returns a user's email. Implemented by auth.Accounts.
type AccountReader interface {
	AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Config holds onboarding settings from the environment.
type Config struct {
	// AppBaseURL builds links in reminder emails.
	AppBaseURL string
}

// Service keeps setup progress, the checklist, per-member UI state and reminders.
type Service struct {
	pool       *pgxpool.Pool
	workspaces Workspaces
	channels   Channels
	accounts   AccountReader
	queue      Enqueuer
	cfg        Config
	now        func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, workspaces Workspaces, channels Channels, accounts AccountReader, queue Enqueuer, cfg Config) *Service {
	return &Service{
		pool: pool, workspaces: workspaces, channels: channels, accounts: accounts, queue: queue, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Start creates a new workspace's progress row and schedules its reminders.
// workspaces.Service calls it inside the create-workspace transaction.
// Exploring with sample data counts as finishing setup: the dashboard opens and
// its banner leads back to the steps.
func (s *Service) Start(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, firstGoal string, sample bool, trialEndsAt time.Time) error {
	now := s.now()
	var completedAt *time.Time
	if sample {
		completedAt = &now
	}
	if err := repository.NewRepository(tx).CreateOnboarding(ctx, gen.CreateOnboardingParams{
		WorkspaceID: workspaceID, FirstGoal: firstGoal, SampleData: sample, CompletedAt: completedAt,
	}); err != nil {
		return fmt.Errorf("create onboarding: %w", err)
	}
	for i, d := range model.SetupReminderDelays {
		if err := s.schedule(ctx, tx, model.SetupReminderArgs{WorkspaceID: workspaceID, N: i + 1}, now.Add(d)); err != nil {
			return err
		}
	}
	for _, days := range model.TrialReminderDays {
		at := trialEndsAt.AddDate(0, 0, -days)
		if at.After(now) {
			if err := s.schedule(ctx, tx, model.TrialReminderArgs{WorkspaceID: workspaceID, DaysLeft: days}, at); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) schedule(ctx context.Context, tx pgx.Tx, args river.JobArgs, at time.Time) error {
	if _, err := s.queue.InsertTx(ctx, tx, args, &river.InsertOpts{ScheduledAt: at}); err != nil {
		return fmt.Errorf("schedule %s: %w", args.Kind(), err)
	}
	return nil
}

// MarkBusinessDone records setup step 1. workspaces.Service calls it in the
// transaction that saves the details.
func (s *Service) MarkBusinessDone(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) error {
	if err := repository.NewRepository(db).MarkBusinessDone(ctx, workspaceID); err != nil {
		return fmt.Errorf("mark business done: %w", err)
	}
	return nil
}

// Progress returns the current step and whether setup is finished or skipped.
func (s *Service) Progress(ctx context.Context, workspaceID uuid.UUID) (step string, completed bool, err error) {
	row, connected, err := s.load(ctx, workspaceID)
	if err != nil {
		return "", false, err
	}
	return currentStep(row, connected), row.CompletedAt != nil, nil
}

// Get returns the workspace's progress, the checklist and this member's UI state.
func (s *Service) Get(ctx context.Context, workspaceID, userID uuid.UUID) (model.Onboarding, error) {
	row, connected, err := s.load(ctx, workspaceID)
	if err != nil {
		return model.Onboarding{}, err
	}
	trialEnds, activated, err := s.workspaces.Trial(ctx, workspaceID)
	if err != nil {
		return model.Onboarding{}, err
	}
	member, err := repository.NewRepository(s.pool).GetMemberState(ctx, gen.GetMemberStateParams{WorkspaceID: workspaceID, UserID: userID})
	if err != nil && !repository.IsNoRows(err) {
		return model.Onboarding{}, fmt.Errorf("member state: %w", err)
	}

	step := currentStep(row, connected)
	skipped := row.WhatsappSkippedAt != nil && !connected
	reached := len(model.Steps)
	if i := slices.Index(model.Steps, step); i >= 0 {
		reached = i
	}
	states := make([]model.StepState, len(model.Steps))
	for i, id := range model.Steps {
		status := model.StatusUpcoming
		switch {
		case skipped && id != model.StepBusiness:
			status = model.StatusSkipped
		case i < reached:
			status = model.StatusDone
		case i == reached:
			status = model.StatusCurrent
		}
		states[i] = model.StepState{ID: id, Status: status}
	}
	return model.Onboarding{
		WorkspaceID: workspaceID, CurrentStep: step, Steps: states,
		FirstGoal: row.FirstGoal, SampleData: row.SampleData, WhatsAppSkipped: skipped,
		Completed: row.CompletedAt != nil, Activated: activated, TrialEndsAt: trialEnds,
		Checklist: []model.ChecklistItem{
			{ID: "account", Done: true},
			{ID: "whatsapp", Done: connected},
			// Done once the Templates, Contacts, Broadcasts and Team features exist.
			{ID: "template"},
			{ID: "contacts"},
			{ID: "broadcast"},
			{ID: "team"},
		},
		TourSeen:        member.TourSeenAt != nil,
		ChecklistHidden: member.ChecklistHiddenAt != nil,
	}, nil
}

// SkipWhatsApp finishes setup without a number ("I'll do this later").
// Only allowed on the WhatsApp step.
func (s *Service) SkipWhatsApp(ctx context.Context, workspaceID, userID uuid.UUID) (model.Onboarding, error) {
	row, connected, err := s.load(ctx, workspaceID)
	if err != nil {
		return model.Onboarding{}, err
	}
	if currentStep(row, connected) != model.StepWhatsApp {
		return model.Onboarding{}, model.ErrStepInvalid
	}
	if err := repository.NewRepository(s.pool).SkipWhatsAppStep(ctx, workspaceID); err != nil {
		return model.Onboarding{}, fmt.Errorf("skip whatsapp: %w", err)
	}
	return s.Get(ctx, workspaceID, userID)
}

// Complete finishes setup from the test message step ("Go to dashboard"). The
// test message itself is optional; a connected number is not.
func (s *Service) Complete(ctx context.Context, workspaceID, userID uuid.UUID) (model.Onboarding, error) {
	connected, err := s.channels.HasConnectedNumber(ctx, workspaceID)
	if err != nil {
		return model.Onboarding{}, err
	}
	if !connected {
		return model.Onboarding{}, model.ErrNotConnected
	}
	if err := repository.NewRepository(s.pool).CompleteOnboarding(ctx, workspaceID); err != nil {
		return model.Onboarding{}, fmt.Errorf("complete onboarding: %w", err)
	}
	return s.Get(ctx, workspaceID, userID)
}

// UpdateMemberState records that this member saw the tour or hid the checklist.
func (s *Service) UpdateMemberState(ctx context.Context, workspaceID, userID uuid.UUID, in model.MemberStateInput) (model.Onboarding, error) {
	if err := repository.NewRepository(s.pool).UpsertMemberState(ctx, gen.UpsertMemberStateParams{
		WorkspaceID: workspaceID, UserID: userID, TourSeen: in.TourSeen, ChecklistHidden: in.ChecklistHidden,
	}); err != nil {
		return model.Onboarding{}, fmt.Errorf("member state: %w", err)
	}
	return s.Get(ctx, workspaceID, userID)
}

// TestMessageDelivered records that the setup test message reached the phone.
// features/whatsapp calls it from the status webhook.
func (s *Service) TestMessageDelivered(ctx context.Context, workspaceID uuid.UUID) error {
	if err := repository.NewRepository(s.pool).MarkTestMessageSent(ctx, workspaceID); err != nil {
		return fmt.Errorf("mark test message: %w", err)
	}
	return nil
}

func (s *Service) load(ctx context.Context, workspaceID uuid.UUID) (gen.GetOnboardingRow, bool, error) {
	row, err := repository.NewRepository(s.pool).GetOnboarding(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return row, false, model.ErrNotFound
	}
	if err != nil {
		return row, false, fmt.Errorf("get onboarding: %w", err)
	}
	connected, err := s.channels.HasConnectedNumber(ctx, workspaceID)
	if err != nil {
		return row, false, err
	}
	return row, connected, nil
}

// currentStep is the first unfinished step. It is separate from "completed":
// skipping WhatsApp or exploring sample data opens the dashboard while a step
// is still open for later.
func currentStep(row gen.GetOnboardingRow, connected bool) string {
	switch {
	case row.BusinessDoneAt == nil:
		return model.StepBusiness
	case !connected:
		return model.StepWhatsApp
	case row.CompletedAt == nil:
		return model.StepTestMessage
	default:
		return model.StepDone
	}
}

// --- Reminders ---

// SendSetupReminder emails the owner if no WhatsApp number is connected yet.
func (s *Service) SendSetupReminder(ctx context.Context, workspaceID uuid.UUID) error {
	row, connected, err := s.load(ctx, workspaceID)
	if err != nil {
		return err
	}
	if connected || int(row.ReminderCount) >= len(model.SetupReminderDelays) {
		return nil
	}
	name, ownerID, err := s.workspaces.Owner(ctx, workspaceID)
	if err != nil {
		return err
	}
	email, _, _, err := s.accounts.AccountInfo(ctx, ownerID)
	if err != nil {
		return err
	}
	msg := setupReminderEmail(email, name, s.link(workspaceID, "/setup"))
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := repository.NewRepository(tx).BumpReminderCount(ctx, workspaceID); err != nil {
			return fmt.Errorf("bump reminders: %w", err)
		}
		if _, err := s.queue.InsertTx(ctx, tx, mailer.SendArgs{Message: msg}, nil); err != nil {
			return fmt.Errorf("queue email: %w", err)
		}
		return nil
	})
}

// SendTrialReminder emails the owner that the trial ends soon.
func (s *Service) SendTrialReminder(ctx context.Context, workspaceID uuid.UUID, daysLeft int) error {
	endsAt, _, err := s.workspaces.Trial(ctx, workspaceID)
	if err != nil {
		return err
	}
	// Skipped once the trial is gone (a plan was chosen in Phase 6, or it was moved).
	if endsAt == nil || endsAt.Before(s.now()) {
		return nil
	}
	name, ownerID, err := s.workspaces.Owner(ctx, workspaceID)
	if err != nil {
		return err
	}
	email, _, _, err := s.accounts.AccountInfo(ctx, ownerID)
	if err != nil {
		return err
	}
	msg := trialReminderEmail(email, name, daysLeft, s.link(workspaceID, ""))
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := s.queue.InsertTx(ctx, tx, mailer.SendArgs{Message: msg}, nil); err != nil {
			return fmt.Errorf("queue email: %w", err)
		}
		return nil
	})
}

// link is a web app page in the workspace. The app switches to ?workspace= and drops it.
func (s *Service) link(workspaceID uuid.UUID, path string) string {
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + path + "?workspace=" + workspaceID.String()
}
