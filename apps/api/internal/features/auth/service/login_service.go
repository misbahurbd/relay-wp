package service

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// --- Login, refresh, logout ---

// Login checks email and password and starts a session.
func (s *Service) Login(ctx context.Context, rawEmail, password string, meta model.RequestMeta) (model.Tokens, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return model.Tokens{}, model.ErrInvalidCredentials
	}
	key := s.passwordKey(email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return model.Tokens{}, err
	} else if blocked {
		return model.Tokens{}, model.ErrTooManyAttempts
	}
	now := s.now()

	r := repository.NewRepository(s.pool)
	user, err := r.GetUserForLogin(ctx, email)
	if err != nil && !repository.IsNoRows(err) {
		return model.Tokens{}, fmt.Errorf("look up user: %w", err)
	}
	if err != nil || user.PasswordHash == nil {
		// Unknown email or social-only account: spend the same time, give the same answer.
		_, _, _ = verifyPassword(password, s.dummyHash)
		var uid *uuid.UUID
		if err == nil {
			uid = &user.ID
		}
		s.recordFailedLogin(ctx, uid, meta, "invalid_credentials", key)
		return model.Tokens{}, model.ErrInvalidCredentials
	}

	ok, rehash, err := verifyPassword(password, *user.PasswordHash)
	if err != nil {
		return model.Tokens{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &user.ID, meta, "invalid_credentials", key)
		return model.Tokens{}, model.ErrInvalidCredentials
	}
	// Only now, with the right password, is it safe to say why sign-in can't proceed.
	if user.Status != "active" {
		s.recordFailedLogin(ctx, &user.ID, meta, "account_disabled", "")
		return model.Tokens{}, model.ErrAccountDisabled
	}
	if user.EmailVerifiedAt == nil {
		s.recordFailedLogin(ctx, &user.ID, meta, "email_not_verified", "")
		return model.Tokens{}, model.ErrEmailNotVerified
	}

	var tokens model.Tokens
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if rehash {
			if h, err := hashPassword(password); err == nil {
				if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: user.ID, PasswordHash: &h}); err != nil {
					return fmt.Errorf("upgrade password hash: %w", err)
				}
			}
		}
		if tokens, err = s.startSession(ctx, r, user.ID, model.ProviderPassword, meta, now); err != nil {
			return err
		}
		if err := r.TouchLastLogin(ctx, user.ID); err != nil {
			return fmt.Errorf("touch last login: %w", err)
		}
		// The key restarts the wrong-password count (see attempts.go).
		return s.record(ctx, r, &user.ID, model.EventLoginSucceeded, model.ProviderPassword, meta, map[string]any{"key": key})
	})
	return tokens, err
}
