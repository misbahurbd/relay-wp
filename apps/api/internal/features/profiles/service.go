package profiles

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// AccountReader returns safe account fields. Implemented by auth.Accounts and injected by internal/app.
type AccountReader interface {
	AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error)
}

// Service manages profiles and preferences.
type Service struct {
	pool     *pgxpool.Pool
	accounts AccountReader
}

// New builds the service.
func New(pool *pgxpool.Pool, accounts AccountReader) *Service {
	return &Service{pool: pool, accounts: accounts}
}

const defaultLocale = "en"

// CreateForUser creates the profile and preferences rows. Auth calls it inside
// the signup transaction, so db is that transaction.
func (s *Service) CreateForUser(ctx context.Context, db database.DBTX, userID uuid.UUID, firstName string, lastName, avatarURL *string, timezone string) error {
	r := newRepository(db)
	if err := r.CreateProfile(ctx, gen.CreateProfileParams{
		UserID: userID, FirstName: firstName, LastName: lastName, AvatarUrl: avatarURL,
	}); err != nil {
		return fmt.Errorf("create profile: %w", err)
	}
	if err := r.CreatePreferences(ctx, gen.CreatePreferencesParams{
		UserID: userID, Locale: defaultLocale, Timezone: timezone,
	}); err != nil {
		return fmt.Errorf("create preferences: %w", err)
	}
	return nil
}

// Me returns the signed-in user's account summary, profile and preferences.
func (s *Service) Me(ctx context.Context, userID uuid.UUID) (Me, error) {
	email, verified, hasPassword, err := s.accounts.AccountInfo(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Me{}, ErrNotFound
		}
		return Me{}, err
	}
	p, err := newRepository(s.pool).GetProfile(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Me{}, ErrNotFound
	}
	if err != nil {
		return Me{}, fmt.Errorf("load profile: %w", err)
	}
	return Me{
		UserID: userID, Email: email, EmailVerified: verified, HasPassword: hasPassword,
		FirstName: p.FirstName, LastName: p.LastName, DisplayName: p.DisplayName, AvatarURL: p.AvatarUrl,
		Locale: p.Locale, Timezone: p.Timezone,
	}, nil
}

// UpdateProfile changes name fields and returns the updated Me.
func (s *Service) UpdateProfile(ctx context.Context, userID uuid.UUID, in ProfileUpdate) (Me, error) {
	if in.FirstName != nil {
		t := strings.TrimSpace(*in.FirstName)
		if t == "" {
			return Me{}, ErrInvalidInput.WithDetails(apperr.Detail{Field: "first_name", Message: "First name can't be empty"})
		}
		in.FirstName = &t
	}
	in.LastName, in.DisplayName = trimPtr(in.LastName), trimPtr(in.DisplayName)

	if err := newRepository(s.pool).UpdateProfile(ctx, gen.UpdateProfileParams{
		UserID: userID, FirstName: in.FirstName, LastName: in.LastName, DisplayName: in.DisplayName,
	}); err != nil {
		return Me{}, fmt.Errorf("update profile: %w", err)
	}
	return s.Me(ctx, userID)
}

var localePattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// UpdatePreferences changes locale and timezone and returns the updated Me.
func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, in PreferencesUpdate) (Me, error) {
	var details []apperr.Detail
	if in.Locale != nil && !localePattern.MatchString(*in.Locale) {
		details = append(details, apperr.Detail{Field: "locale", Message: "Use a language tag like en or en-GB"})
	}
	if in.Timezone != nil {
		if _, err := time.LoadLocation(*in.Timezone); err != nil || *in.Timezone == "" || *in.Timezone == "Local" {
			details = append(details, apperr.Detail{Field: "timezone", Message: "Unknown timezone"})
		}
	}
	if details != nil {
		return Me{}, ErrInvalidInput.WithDetails(details...)
	}

	if err := newRepository(s.pool).UpdatePreferences(ctx, gen.UpdatePreferencesParams{
		UserID: userID, Locale: in.Locale, Timezone: in.Timezone,
	}); err != nil {
		return Me{}, fmt.Errorf("update preferences: %w", err)
	}
	return s.Me(ctx, userID)
}

// trimPtr trims a provided value; an empty result still clears the field.
func trimPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
