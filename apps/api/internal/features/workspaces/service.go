// Package workspaces owns workspaces (the businesses using Relay) and their members.
package workspaces

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

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
	Start(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, firstGoal string, sample bool, trialEndsAt time.Time) error
	// MarkBusinessDone records setup step 1 in the same transaction as the details.
	MarkBusinessDone(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) error
	// Progress returns the current setup step and whether the dashboard has opened.
	Progress(ctx context.Context, workspaceID uuid.UUID) (step string, completed bool, err error)
}

// Config holds workspace settings from the environment.
type Config struct {
	TrialDays int
}

// Service manages workspaces and their members.
type Service struct {
	pool       *pgxpool.Pool
	accounts   AccountReader
	onboarding Onboarding
	cfg        Config
	now        func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, accounts AccountReader, onboarding Onboarding, cfg Config) *Service {
	return &Service{
		pool: pool, accounts: accounts, onboarding: onboarding, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// List returns the user's workspaces, most recently used first.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Summary, error) {
	rows, err := newRepository(s.pool).ListMemberWorkspaces(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		step, completed, err := s.onboarding.Progress(ctx, r.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, Summary{
			ID: r.ID, Name: r.Name, Role: r.Role, LastActiveAt: r.LastActiveAt,
			CurrentStep: step, Completed: completed, Activated: r.ActivatedAt != nil,
		})
	}
	return out, nil
}

// Create makes a workspace from the Welcome screen: the workspace, the owner's
// membership, the trial and the onboarding row, in one transaction.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (Summary, error) {
	name := strings.TrimSpace(in.Name)
	var details []apperr.Detail
	if name == "" || utf8.RuneCountInString(name) > 100 {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter a name of up to 100 characters"})
	}
	if !slices.Contains(firstGoals, in.FirstGoal) {
		details = append(details, apperr.Detail{Field: "first_goal", Message: "Choose what you want to do first"})
	}
	if in.StartMode != StartSetup && in.StartMode != StartSample {
		details = append(details, apperr.Detail{Field: "start_mode", Message: "Choose setup or sample"})
	}
	if len(details) > 0 {
		return Summary{}, ErrInvalidInput.WithDetails(details...)
	}

	_, verified, _, err := s.accounts.AccountInfo(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	if !verified {
		return Summary{}, ErrEmailNotVerified
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Summary{}, fmt.Errorf("workspace id: %w", err)
	}
	now := s.now()
	trialEnds := now.AddDate(0, 0, s.cfg.TrialDays)
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		// Serialize a user's creates so two tabs can't both pass the limit check.
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))", userID); err != nil {
			return fmt.Errorf("lock user: %w", err)
		}
		owned, err := r.CountTrialWorkspacesOwned(ctx, userID)
		if err != nil {
			return fmt.Errorf("count workspaces: %w", err)
		}
		if owned > 0 {
			return ErrLimitReached
		}
		if err := r.CreateWorkspace(ctx, gen.CreateWorkspaceParams{ID: id, Name: name, TrialEndsAt: &trialEnds, CreatedBy: &userID}); err != nil {
			return fmt.Errorf("create workspace: %w", err)
		}
		if err := r.AddWorkspaceMember(ctx, gen.AddWorkspaceMemberParams{WorkspaceID: id, UserID: userID, Role: RoleOwner}); err != nil {
			return fmt.Errorf("add owner: %w", err)
		}
		return s.onboarding.Start(ctx, tx, id, in.FirstGoal, in.StartMode == StartSample, trialEnds)
	})
	if err != nil {
		return Summary{}, err
	}
	step, completed, err := s.onboarding.Progress(ctx, id)
	if err != nil {
		return Summary{}, err
	}
	return Summary{ID: id, Name: name, Role: RoleOwner, LastActiveAt: now, CurrentStep: step, Completed: completed}, nil
}

// Get returns one workspace and marks it as the user's most recently used.
// The caller is a member (middleware.RequireWorkspace checked).
func (s *Service) Get(ctx context.Context, workspaceID, userID uuid.UUID) (Workspace, error) {
	r := newRepository(s.pool)
	row, err := r.GetWorkspace(ctx, workspaceID)
	if isNoRows(err) {
		return Workspace{}, ErrNotFound
	}
	if err != nil {
		return Workspace{}, fmt.Errorf("get workspace: %w", err)
	}
	if err := r.TouchWorkspaceMember(ctx, gen.TouchWorkspaceMemberParams{WorkspaceID: workspaceID, UserID: userID}); err != nil {
		return Workspace{}, fmt.Errorf("touch member: %w", err)
	}
	return toWorkspace(row), nil
}

// SaveBusiness saves setup step 1 on the workspace and marks the step done. It
// can be called again (the Back button), and later from Settings.
func (s *Service) SaveBusiness(ctx context.Context, workspaceID uuid.UUID, in BusinessInput) (Workspace, error) {
	params, err := businessParams(workspaceID, in)
	if err != nil {
		return Workspace{}, err
	}
	var out Workspace
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
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

func businessParams(workspaceID uuid.UUID, in BusinessInput) (gen.UpdateWorkspaceBusinessParams, error) {
	var details []apperr.Detail
	name := strings.TrimSpace(in.Name)
	if name == "" || utf8.RuneCountInString(name) > 100 {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter your business name (up to 100 characters)"})
	}
	if !slices.Contains(industries, in.Industry) {
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
	if !slices.Contains(teamSizes, in.TeamSize) {
		details = append(details, apperr.Detail{Field: "team_size", Message: "Choose your team size"})
	}
	goals := make([]string, 0, len(in.Goals))
	for _, g := range in.Goals {
		if !slices.Contains(goalValues, g) {
			details = append(details, apperr.Detail{Field: "goals", Message: "Choose from marketing, support and sales"})
			break
		}
		if !slices.Contains(goals, g) {
			goals = append(goals, g)
		}
	}
	if len(details) > 0 {
		return gen.UpdateWorkspaceBusinessParams{}, ErrInvalidInput.WithDetails(details...)
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

func toWorkspace(r gen.GetWorkspaceRow) Workspace {
	return Workspace{
		ID: r.ID, Name: r.Name, Industry: r.Industry, Website: r.Website, Country: r.Country,
		TeamSize: r.TeamSize, Goals: r.Goals, Plan: r.Plan, TrialEndsAt: r.TrialEndsAt, ActivatedAt: r.ActivatedAt,
	}
}
