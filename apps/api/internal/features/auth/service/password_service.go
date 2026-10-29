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
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// --- Passwords ---

// ForgotPassword emails a reset link if the account exists. Always returns nil
// for unknown emails.
func (s *Service) ForgotPassword(ctx context.Context, rawEmail string, meta model.RequestMeta) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		acc, err := r.GetAccountByEmail(ctx, email)
		if repository.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("look up email: %w", err)
		}
		if acc.Status != "active" {
			return nil
		}
		raw, err := s.issueEmailToken(ctx, r, acc.ID, acc.Email, model.PurposeReset, model.ResetTokenTTL)
		if err != nil {
			return err
		}
		if err := s.enqueue(ctx, tx, resetPasswordEmail(acc.Email, s.link("/reset-password", raw))); err != nil {
			return err
		}
		return s.record(ctx, r, &acc.ID, model.EventPasswordResetRequested, "", meta, nil)
	})
}

// ResetPassword sets a new password from an emailed link, marks the email
// verified (the link proves ownership) and signs out every device.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string, meta model.RequestMeta) error {
	hash, err := s.newPasswordHash(ctx, newPassword)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		row, err := r.ConsumeEmailToken(ctx, gen.ConsumeEmailTokenParams{TokenHash: hashToken(token), Purpose: model.PurposeReset})
		if repository.IsNoRows(err) {
			return model.ErrInvalidLink
		}
		if err != nil {
			return fmt.Errorf("consume reset token: %w", err)
		}
		acc, err := r.GetAccount(ctx, row.UserID)
		if err != nil {
			return fmt.Errorf("load account: %w", err)
		}
		if acc.EmailVerifiedAt == nil {
			// The link proves who owns this email. A social account the provider
			// didn't vouch for (Facebook) may have been someone else's: disconnect it.
			n, err := r.DeleteUnverifiedIdentities(ctx, row.UserID)
			if err != nil {
				return fmt.Errorf("remove unverified identities: %w", err)
			}
			if n > 0 {
				if err := s.record(ctx, r, &row.UserID, model.EventIdentityUnlinked, "", meta,
					map[string]any{"reason": "password_reset_unverified"}); err != nil {
					return err
				}
			}
		}
		if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: row.UserID, PasswordHash: &hash}); err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if err := r.MarkEmailVerified(ctx, row.UserID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		reason := model.RevokePasswordReset
		if _, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: row.UserID, Reason: &reason}); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return s.record(ctx, r, &row.UserID, model.EventPasswordResetCompleted, "", meta, nil)
	})
}

// ChangePassword replaces the password after checking the current one, and
// signs out every other device.
func (s *Service) ChangePassword(ctx context.Context, userID, sessionID uuid.UUID, current, next string, meta model.RequestMeta) error {
	r := repository.NewRepository(s.pool)
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	stored, err := r.GetPasswordHash(ctx, userID)
	if err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if stored == nil {
		return model.ErrNoPassword
	}
	// A stolen session must not be a way to guess the password without limits.
	key := s.passwordKey(acc.Email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return err
	} else if blocked {
		return model.ErrTooManyAttempts
	}
	ok, _, err := verifyPassword(current, *stored)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &userID, meta, "wrong_current_password", key)
		return model.ErrWrongPassword.WithDetails(apperr.Detail{Field: "current_password", Message: "Current password is incorrect"})
	}
	hash, err := s.newPasswordHash(ctx, next)
	if err != nil {
		return err
	}

	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: userID, PasswordHash: &hash}); err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		reason := model.RevokePasswordChanged
		if _, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: userID, Reason: &reason, KeepID: &sessionID}); err != nil {
			return fmt.Errorf("revoke other sessions: %w", err)
		}
		return s.record(ctx, r, &userID, model.EventPasswordChanged, "", meta, nil)
	})
}

// --- helpers ---

// newPasswordHash checks the rules and the breach list, then hashes.
func (s *Service) newPasswordHash(ctx context.Context, pw string) (string, error) {
	if d := checkPasswordRules(pw); d != nil {
		return "", model.ErrWeakPassword.WithDetails(d...)
	}
	if s.breach != nil {
		breached, err := s.breach.IsBreached(ctx, pw)
		if err != nil {
			// Fail open: the breach service being down must not block signups.
			logger.FromContext(ctx).Warn("password breach check unavailable", "err", err)
		}
		if breached {
			return "", model.ErrBreachedPassword.WithDetails(apperr.Detail{Field: "password", Message: model.ErrBreachedPassword.Message})
		}
	}
	return hashPassword(pw)
}

// SetPassword adds a password to an account that signs in only with providers.
// It needs a recent sign-in and a verified email.
func (s *Service) SetPassword(ctx context.Context, userID, sessionID uuid.UUID, password string, meta model.RequestMeta) error {
	hash, err := s.newPasswordHash(ctx, password)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := s.requireRecentSignIn(ctx, r, userID, sessionID); err != nil {
			return err
		}
		acc, err := r.GetAccount(ctx, userID)
		if err != nil {
			return fmt.Errorf("load account: %w", err)
		}
		if acc.EmailVerifiedAt == nil {
			return ErrVerifyEmailFirst
		}
		n, err := r.SetPasswordIfUnset(ctx, gen.SetPasswordIfUnsetParams{ID: userID, PasswordHash: &hash})
		if err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		if n == 0 {
			return model.ErrPasswordAlreadySet
		}
		return s.record(ctx, r, &userID, model.EventPasswordSet, "", meta, nil)
	})
}
