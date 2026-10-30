// Package service holds workspaces' business rules (no HTTP, no SQL). The workspaces feature owns
// workspaces (the businesses using Relay) and their members.
package service

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// AccountReader says whether a user's email is verified. Implemented by auth.Accounts.
type AccountReader interface {
	AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error)
}

// Onboarding keeps setup progress. Implemented by features/onboarding and injected by internal/app.
type Onboarding interface {
	// Start creates the progress row (and schedules reminders) in the create-workspace transaction.
	// trialEndsAt is nil when the workspace has no trial.
	Start(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, firstGoal string, sample bool, trialEndsAt *time.Time) error
	// MarkBusinessDone records setup step 1 in the same transaction as the details.
	MarkBusinessDone(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) error
	// Progress returns the current setup step and whether the dashboard has opened.
	Progress(ctx context.Context, workspaceID uuid.UUID) (step string, completed bool, err error)
}

// Team sets up roles and the owner's membership. Implemented by features/team and injected by internal/app.
type Team interface {
	// Seed creates the system roles and adds ownerID as Owner, in the create-workspace transaction.
	Seed(ctx context.Context, tx pgx.Tx, workspaceID, ownerID uuid.UUID) error
}

// Trials starts a new workspace's free trial. Implemented by features/billing and injected by internal/app.
type Trials interface {
	// StartWorkspaceTrial starts the trial in the create-workspace transaction
	// and returns when it ends; nil means the workspace starts with no trial.
	StartWorkspaceTrial(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (*time.Time, error)
}

// Service manages workspaces and their members.
type Service struct {
	pool       *pgxpool.Pool
	accounts   AccountReader
	onboarding Onboarding
	team       Team
	billing    Billing // nil: deletion is never blocked by billing
	trials     Trials  // nil: new workspaces start with no trial
	now        func() time.Time
}

// SetTrials makes new workspaces start the trial billing gives them.
func (s *Service) SetTrials(t Trials) { s.trials = t }

// New builds the service.
func New(pool *pgxpool.Pool, accounts AccountReader, onboarding Onboarding, team Team) *Service {
	return &Service{
		pool: pool, accounts: accounts, onboarding: onboarding, team: team,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// List returns the user's workspaces, most recently used first.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]model.Summary, error) {
	rows, err := repository.NewRepository(s.pool).ListMemberWorkspaces(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	out := make([]model.Summary, 0, len(rows))
	for _, r := range rows {
		step, completed, err := s.onboarding.Progress(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, model.Summary{
			ID: r.ID, Name: r.Name, RoleName: r.RoleName, IsOwner: r.IsOwner, LastActiveAt: r.LastActiveAt,
			CurrentStep: step, Completed: completed, Activated: r.ActivatedAt != nil,
		})
	}
	return out, nil
}

// Create makes a workspace from the Welcome screen: the workspace, the owner's
// membership, the trial and the onboarding row, in one transaction.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in model.CreateInput) (model.Summary, error) {
	name := strings.TrimSpace(in.Name)
	var details []apperr.Detail
	if name == "" || utf8.RuneCountInString(name) > 100 {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter a name of up to 100 characters"})
	}
	if !slices.Contains(model.FirstGoals, in.FirstGoal) {
		details = append(details, apperr.Detail{Field: "first_goal", Message: "Choose what you want to do first"})
	}
	if in.StartMode != model.StartSetup && in.StartMode != model.StartSample {
		details = append(details, apperr.Detail{Field: "start_mode", Message: "Choose setup or sample"})
	}
	if len(details) > 0 {
		return model.Summary{}, model.ErrInvalidInput.WithDetails(details...)
	}

	_, verified, _, err := s.accounts.AccountInfo(ctx, userID)
	if err != nil {
		return model.Summary{}, err
	}
	if !verified {
		return model.Summary{}, model.ErrEmailNotVerified
	}

	id, err := uuid.NewV7()
	if err != nil {
		return model.Summary{}, fmt.Errorf("workspace id: %w", err)
	}
	now := s.now()
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		// Serialize a user's creates so two tabs can't both pass the limit check.
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		owned, err := r.CountTrialWorkspacesOwned(ctx, userID)
		if err != nil {
			return fmt.Errorf("count workspaces: %w", err)
		}
		if owned > 0 {
			return model.ErrLimitReached
		}
		if err := r.CreateWorkspace(ctx, gen.CreateWorkspaceParams{ID: id, Name: name, CreatedBy: &userID}); err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
		if err := s.team.Seed(ctx, tx, id, userID); err != nil {
			return fmt.Errorf("seed team: %w", err)
		}
		var trialEnds *time.Time
		if s.trials != nil {
			if trialEnds, err = s.trials.StartWorkspaceTrial(ctx, tx, id); err != nil {
				return fmt.Errorf("start trial: %w", err)
			}
		}
		return s.onboarding.Start(ctx, tx, id, in.FirstGoal, in.StartMode == model.StartSample, trialEnds)
	})
	if err != nil {
		return model.Summary{}, err
	}
	step, completed, err := s.onboarding.Progress(ctx, id)
	if err != nil {
		return model.Summary{}, err
	}
	return model.Summary{ID: id, Name: name, RoleName: "Owner", IsOwner: true, LastActiveAt: now, CurrentStep: step, Completed: completed}, nil
}

// Get returns one workspace and marks it as the user's most recently used.
// The caller is a member (middleware.RequireWorkspace checked).
func (s *Service) Get(ctx context.Context, workspaceID, userID uuid.UUID) (model.Workspace, error) {
	r := repository.NewRepository(s.pool)
	row, err := r.GetWorkspace(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return model.Workspace{}, model.ErrNotFound
	}
	if err != nil {
		return model.Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	if err := r.TouchWorkspaceMember(ctx, gen.TouchWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID}); err != nil {
		return model.Workspace{}, fmt.Errorf("touch member: %w", err)
	}
	return toWorkspace(row), nil
}

// SaveBusiness saves setup step 1 on the workspace and marks the step done. It
// can be called again (the Back button), and later from Settings.
func (s *Service) SaveBusiness(ctx context.Context, workspaceID uuid.UUID, in model.BusinessInput) (model.Workspace, error) {
	params, err := businessParams(workspaceID, in)
	if err != nil {
		return model.Workspace{}, err
	}
	var out model.Workspace
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := r.UpdateWorkspaceBusiness(ctx, params); err != nil {
			return fmt.Errorf("save business: %w", err)
		}
		if err := s.onboarding.MarkBusinessDone(ctx, tx, workspaceID); err != nil {
			return err
		}
		row, err := r.GetWorkspace(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("get workspace: %w", err)
		}
		out = toWorkspace(row)
		return nil
	})
	return out, err
}

var countryRe = regexp.MustCompile(`^[A-Z]{2}$`)

func businessParams(workspaceID uuid.UUID, in model.BusinessInput) (gen.UpdateWorkspaceBusinessParams, error) {
	var details []apperr.Detail
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter your business name (up to 100 characters)"})
	}
	if !slices.Contains(model.Industries, in.Industry) {
		details = append(details, apperr.Detail{Field: "industry", Message: "Choose your industry"})
	}
	website, ok := normalizeWebsite(in.Website)
	if !ok {
		details = append(details, apperr.Detail{Field: "website", Message: "Enter a web address like yourbusiness.com"})
	}
	country := strings.ToUpper(strings.TrimSpace(in.Country))
	if !countryRe.MatchString(country) {
		details = append(details, apperr.Detail{Field: "country", Message: "Choose your country"})
	}
	if !slices.Contains(model.TeamSizes, in.TeamSize) {
		details = append(details, apperr.Detail{Field: "team_size", Message: "Choose your team size"})
	}
	goals := make([]string, 0, len(in.Goals))
	for _, g := range in.Goals {
		if !slices.Contains(model.GoalValues, g) {
			details = append(details, apperr.Detail{Field: "goals", Message: "Choose from marketing, support and sales"})
			break
		}
		if !slices.Contains(goals, g) {
			goals = append(goals, g)
		}
	}
	if len(details) > 0 {
		return gen.UpdateWorkspaceBusinessParams{}, model.ErrInvalidInput.WithDetails(details...)
	}
	return gen.UpdateWorkspaceBusinessParams{
		WorkspaceID: workspaceID, Name: name, Industry: &in.Industry, Website: website,
		Country: &country, TeamSize: &in.TeamSize, Goals: goals,
	}, nil
}

// normalizeWebsite accepts "shop.com" or a full URL and stores an https URL.
// Empty is fine: the field is optional.
func normalizeWebsite(raw string) (*string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	if len(raw) > 200 {
		return nil, false
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.Contains(u.Hostname(), ".") || u.User != nil {
		return nil, false
	}
	s := u.String()
	return &s, true
}

func toWorkspace(r gen.GetWorkspaceRow) model.Workspace {
	return model.Workspace{
		ID: r.ID, Name: r.Name, Industry: r.Industry, Website: r.Website, Country: r.Country,
		TeamSize: r.TeamSize, Goals: r.Goals, Plan: r.Plan, TrialEndsAt: r.TrialEndsAt, ActivatedAt: r.ActivatedAt,
	}
}
