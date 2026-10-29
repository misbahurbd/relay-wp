package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Invites checks invite links and joins their workspace. Implemented by
// features/team and injected by internal/app.
type Invites interface {
	// InviteEmail returns the address a usable invite link was sent to.
	InviteEmail(ctx context.Context, token string) (string, error)
	// JoinByInvite adds userID to the invite's workspace.
	JoinByInvite(ctx context.Context, token string, userID uuid.UUID, ip, userAgent string) (uuid.UUID, error)
}

// ErrInviteHasAccount is an invite sign-up for an email that already has an account.
var ErrInviteHasAccount = apperr.Conflict("account_exists", "You already have an account with this email. Log in to accept the invite.")

// InviteSignupInput is the Accept invite form for someone new to Relay.
type InviteSignupInput struct {
	Token       string
	Password    string
	FirstName   string
	LastName    *string
	Timezone    string
	AcceptTerms bool
}

// SignupFromInvite creates a password account for the invited address, signs it
// in and joins the workspace. The emailed link already proves the inbox is
// theirs, so there is no separate verification step.
func (s *Service) SignupFromInvite(ctx context.Context, in InviteSignupInput, meta model.RequestMeta) (model.Tokens, uuid.UUID, error) {
	rawEmail, err := s.invites.InviteEmail(ctx, in.Token)
	if err != nil {
		return model.Tokens{}, uuid.Nil, err
	}
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return model.Tokens{}, uuid.Nil, err
	}
	firstName := strings.TrimSpace(in.FirstName)
	if firstName == "" {
		return model.Tokens{}, uuid.Nil, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "first_name", Message: "Enter your name"})
	}
	if !in.AcceptTerms {
		return model.Tokens{}, uuid.Nil, model.ErrTermsRequired
	}
	tz, err := normalizeTimezone(in.Timezone)
	if err != nil {
		return model.Tokens{}, uuid.Nil, err
	}
	hash, err := s.newPasswordHash(ctx, in.Password)
	if err != nil {
		return model.Tokens{}, uuid.Nil, err
	}

	var tokens model.Tokens
	var userID uuid.UUID
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if _, err := r.GetAccountByEmail(ctx, email); err == nil {
			return ErrInviteHasAccount
		} else if !repository.IsNoRows(err) {
			return fmt.Errorf("look up email: %w", err)
		}
		if userID, err = uuid.NewV7(); err != nil {
			return fmt.Errorf("user id: %w", err)
		}
		if err := r.CreateUser(ctx, gen.CreateUserParams{ID: userID, Email: email, PasswordHash: &hash}); err != nil {
			if repository.IsUniqueViolation(err) {
				return ErrInviteHasAccount
			}
			return fmt.Errorf("create user: %w", err)
		}
		if err := r.SetTermsAccepted(ctx, gen.SetTermsAcceptedParams{ID: userID, TermsVersion: &s.cfg.TermsVersion}); err != nil {
			return fmt.Errorf("record terms: %w", err)
		}
		if err := s.profiles.CreateForUser(ctx, tx, userID, firstName, trimmedOrNil(in.LastName), nil, tz); err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		if err := r.MarkEmailVerified(ctx, userID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		if err := s.record(ctx, r, &userID, model.EventSignup, model.ProviderPassword, meta, map[string]any{"via": "invite"}); err != nil {
			return err
		}
		if tokens, err = s.startSession(ctx, r, userID, model.ProviderPassword, meta, s.now()); err != nil {
			return err
		}
		if err := r.TouchLastLogin(ctx, userID); err != nil {
			return fmt.Errorf("touch last login: %w", err)
		}
		return s.record(ctx, r, &userID, model.EventLoginSucceeded, model.ProviderPassword, meta,
			map[string]any{"key": s.passwordKey(email)})
	})
	if err != nil {
		return model.Tokens{}, uuid.Nil, err
	}
	// The account stands even if joining fails (say the invite was revoked a
	// moment ago); the person is signed in and sees why.
	workspaceID, err := s.invites.JoinByInvite(ctx, in.Token, userID, meta.IP, meta.UserAgent)
	return tokens, workspaceID, err
}
