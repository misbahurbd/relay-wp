// Package service holds auth's business rules (no HTTP, no SQL). The auth feature owns sign-in:
// users, password and email-link flows, sessions, refresh tokens and the security event log.
// Display data lives in features/profiles.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// TokenIssuer signs access tokens (platform/jwt).
type TokenIssuer interface {
	Issue(userID, sessionID uuid.UUID, now time.Time) (string, time.Time, error)
}

// Enqueuer inserts background jobs inside a transaction (River).
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// ProfileCreator creates a new user's profile and preferences in the signup
// transaction. Implemented by features/profiles and injected by internal/app.
type ProfileCreator interface {
	CreateForUser(ctx context.Context, db database.DBTX, userID uuid.UUID, firstName string, lastName, avatarURL *string, timezone string) error
}

// Config holds the values the service needs from configuration.
type Config struct {
	RefreshTTL time.Duration
	// SessionMaxAge caps a session's life however often it refreshes.
	SessionMaxAge time.Duration
	AppBaseURL    string

	// APIBaseURL builds OAuth callback URLs.
	APIBaseURL string
	// AllowedRedirects are extra exact URLs a social sign-in may return to
	// (e.g. mobile deep links). Any page on AppBaseURL is always allowed.
	AllowedRedirects []string
	// ReauthMaxAge is how recent a sign-in must be for sensitive actions.
	ReauthMaxAge time.Duration
	// EmailCodeKey keys the HMAC of sign-up verification codes.
	EmailCodeKey []byte
	// TermsVersion is stored with each sign-up's acceptance of the terms.
	TermsVersion string
}

// Service implements every auth flow. Handlers call it; it never sees HTTP.
type Service struct {
	pool     *pgxpool.Pool
	tokens   TokenIssuer
	queue    Enqueuer
	profiles ProfileCreator
	breach   BreachChecker // nil disables the breach check
	oauth    Providers     // only configured providers
	invites  Invites
	cfg      Config

	dummyHash string // verified against for unknown emails so timing doesn't reveal them
	now       func() time.Time
}

// New builds the service. breach may be nil; providers may be empty.
func New(pool *pgxpool.Pool, tokens TokenIssuer, queue Enqueuer, profiles ProfileCreator, invites Invites, breach BreachChecker, providers Providers, cfg Config) (*Service, error) {
	if len(cfg.EmailCodeKey) != 32 {
		return nil, errors.New("auth: EMAIL_CODE_KEY must be 32 bytes")
	}
	dummy, err := HashPassword("not-a-real-password-Aa1!")
	if err != nil {
		return nil, err
	}
	return &Service{
		pool: pool, tokens: tokens, queue: queue, profiles: profiles, invites: invites, breach: breach, oauth: providers, cfg: cfg,
		dummyHash: dummy,
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

// issueEmailToken invalidates earlier unused links of the same purpose and creates a new one.
func (s *Service) issueEmailToken(ctx context.Context, r repository.Repository, userID uuid.UUID, email, purpose string, ttl time.Duration) (string, error) {
	if err := r.InvalidateEmailTokens(ctx, gen.InvalidateEmailTokensParams{UserID: userID, Purpose: purpose}); err != nil {
		return "", fmt.Errorf("invalidate old links: %w", err)
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("email token id: %w", err)
	}
	if err := r.CreateEmailToken(ctx, gen.CreateEmailTokenParams{
		ID: id, UserID: userID, Purpose: purpose, Email: email, TokenHash: hash, ExpiresAt: s.now().Add(ttl),
	}); err != nil {
		return "", fmt.Errorf("create email token: %w", err)
	}
	return raw, nil
}

func (s *Service) enqueue(ctx context.Context, tx pgx.Tx, msg mailer.Message) error {
	if _, err := s.queue.InsertTx(ctx, tx, mailer.SendArgs{Message: msg}, nil); err != nil {
		return fmt.Errorf("queue email: %w", err)
	}
	return nil
}

func (s *Service) link(path, token string) string {
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + path + "?token=" + token
}

// record writes one auth_events row. metadata must never contain secrets.
func (s *Service) record(ctx context.Context, r repository.Repository, userID *uuid.UUID, event, provider string, meta model.RequestMeta, metadata map[string]any) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("event id: %w", err)
	}
	md := []byte("{}")
	if metadata != nil {
		if md, err = json.Marshal(metadata); err != nil {
			return fmt.Errorf("event metadata: %w", err)
		}
	}
	var p *string
	if provider != "" {
		p = &provider
	}
	if err := r.InsertAuthEvent(ctx, gen.InsertAuthEventParams{
		ID: id, UserID: userID, Event: event, Provider: p,
		Ip: repository.ParseIP(meta.IP), UserAgent: truncate(meta.UserAgent, 512), Metadata: md,
	}); err != nil {
		return fmt.Errorf("record %s: %w", event, err)
	}
	return nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 {
		return "", model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "email", Message: "Enter a valid email address"})
	}
	return strings.ToLower(email), nil
}

func normalizeTimezone(tz string) (string, error) {
	tz = strings.TrimSpace(tz)
	if tz == "" {
		return "UTC", nil
	}
	if tz == "Local" {
		return "", invalidTimezone()
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return "", invalidTimezone()
	}
	return tz, nil
}

func invalidTimezone() error {
	return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "timezone", Message: "Unknown timezone"})
}

func trimmedOrNil(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
