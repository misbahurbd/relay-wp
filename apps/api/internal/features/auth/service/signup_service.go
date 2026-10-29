package service

import (
	"context"
	"crypto/hmac"
	"fmt"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// --- Signup and email verification ---

// Signup creates an unverified password account and emails a verification link.
// It returns nil whether or not the email is already registered (no enumeration);
// an existing owner gets a different email instead.
func (s *Service) Signup(ctx context.Context, in model.SignupInput, meta model.RequestMeta) error {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return err
	}
	firstName := strings.TrimSpace(in.FirstName)
	if firstName == "" {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "first_name", Message: "Enter your first name"})
	}
	if !in.AcceptTerms {
		return model.ErrTermsRequired
	}
	tz, err := normalizeTimezone(in.Timezone)
	if err != nil {
		return err
	}
	// Hash before looking the email up, so both paths take about the same time.
	hash, err := s.newPasswordHash(ctx, in.Password)
	if err != nil {
		return err
	}

	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)

		existing, err := r.GetAccountByEmail(ctx, email)
		switch {
		case err == nil:
			return s.emailExistingAccount(ctx, tx, r, existing)
		case !repository.IsNoRows(err):
			return fmt.Errorf("look up email: %w", err)
		}

		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("user id: %w", err)
		}
		if err := r.CreateUser(ctx, gen.CreateUserParams{ID: id, Email: email, PasswordHash: &hash}); err != nil {
			if repository.IsUniqueViolation(err) {
				return nil // a concurrent signup won; same response
			}
			return fmt.Errorf("create user: %w", err)
		}
		if err := r.SetTermsAccepted(ctx, gen.SetTermsAcceptedParams{ID: id, TermsVersion: &s.cfg.TermsVersion}); err != nil {
			return fmt.Errorf("record terms: %w", err)
		}
		if err := s.profiles.CreateForUser(ctx, tx, id, firstName, trimmedOrNil(in.LastName), nil, tz); err != nil {
			return fmt.Errorf("create profile: %w", err)
		}
		if err := s.sendVerifyLink(ctx, tx, r, id, email); err != nil {
			return err
		}
		return s.record(ctx, r, &id, model.EventSignup, model.ProviderPassword, meta, nil)
	})
}

// emailExistingAccount tells the real owner someone tried to sign up with their email.
func (s *Service) emailExistingAccount(ctx context.Context, tx pgx.Tx, r repository.Repository, acc gen.GetAccountByEmailRow) error {
	if acc.EmailVerifiedAt == nil {
		// Unverified account: send a set-password link rather than re-verifying, so
		// a password chosen by whoever created the account is never activated.
		raw, err := s.issueEmailToken(ctx, r, acc.ID, acc.Email, model.PurposeReset, model.ResetTokenTTL)
		if err != nil {
			return err
		}
		return s.enqueue(ctx, tx, finishSignupEmail(acc.Email, s.link("/reset-password", raw)))
	}
	return s.enqueue(ctx, tx, alreadyRegisteredEmail(acc.Email, s.cfg.AppBaseURL+"/login"))
}

// VerifyEmail marks the email verified. It does not sign the user in.
//
// The link only proves who owns the inbox. Anyone can sign up with someone
// else's address, so an account with a password must also confirm it here;
// otherwise the owner clicking the link would activate a stranger's password.
// A missing or wrong password rolls back, leaving the link usable.
func (s *Service) VerifyEmail(ctx context.Context, token, password string, meta model.RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		row, err := r.ConsumeEmailToken(ctx, gen.ConsumeEmailTokenParams{TokenHash: hashToken(token), Purpose: model.PurposeVerify})
		if repository.IsNoRows(err) {
			return model.ErrInvalidLink
		}
		if err != nil {
			return fmt.Errorf("consume verify token: %w", err)
		}
		if err := s.confirmPassword(ctx, r, row.UserID, row.Email, password, meta); err != nil {
			return err
		}
		if err := r.MarkEmailVerified(ctx, row.UserID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		return s.record(ctx, r, &row.UserID, model.EventEmailVerified, "", meta, nil)
	})
}

// confirmPassword checks password against the account's own, if it has one.
// Wrong guesses count toward the same limits as login.
func (s *Service) confirmPassword(ctx context.Context, r repository.Repository, userID uuid.UUID, email, password string, meta model.RequestMeta) error {
	stored, err := r.GetPasswordHash(ctx, userID)
	if err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if stored == nil {
		return nil // social-only account: nobody chose a password
	}
	if password == "" {
		return model.ErrPasswordRequired
	}
	key := s.passwordKey(email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return err
	} else if blocked {
		return model.ErrTooManyAttempts
	}
	ok, _, err := verifyPassword(password, *stored)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &userID, meta, "wrong_password", key)
		return model.ErrVerifyWrongPassword
	}
	return nil
}

// VerifyEmailCode checks the 6-digit code from the sign-up email, marks the email
// verified and signs the user in. The code proves who owns the inbox; the
// password proves they also chose it (anyone can sign up with someone else's
// address). The code is checked first, so a password answer never tells someone
// without the code that an account exists.
//
// A wrong code, an expired code and an email with no account waiting all get
// ErrInvalidCode. After verifyCodeMaxAttempts wrong tries the code is dead.
func (s *Service) VerifyEmailCode(ctx context.Context, rawEmail, code, password string, meta model.RequestMeta) (model.Tokens, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return model.Tokens{}, err
	}
	code = strings.TrimSpace(code)
	given := hashEmailCode(s.cfg.EmailCodeKey, code)

	var tokens model.Tokens
	var tooMany bool
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		acc, err := r.GetAccountByEmail(ctx, email)
		if repository.IsNoRows(err) {
			return model.ErrInvalidCode
		}
		if err != nil {
			return fmt.Errorf("look up email: %w", err)
		}
		if acc.EmailVerifiedAt != nil || acc.Status != "active" {
			return model.ErrInvalidCode
		}
		row, err := r.GetActiveEmailCode(ctx, gen.GetActiveEmailCodeParams{UserID: acc.ID, Purpose: model.PurposeVerify})
		if repository.IsNoRows(err) {
			return model.ErrInvalidCode
		}
		if err != nil {
			return fmt.Errorf("load code: %w", err)
		}
		if row.Attempts >= model.VerifyCodeMaxAttempts {
			return model.ErrTooManyCodeAttempts
		}
		if !hmac.Equal(row.CodeHash, given) {
			attempts, err := r.RecordEmailCodeAttempt(ctx, row.ID)
			if err != nil {
				return fmt.Errorf("count attempt: %w", err)
			}
			// Commit the attempt count; the answer is decided after the transaction.
			tooMany = attempts >= model.VerifyCodeMaxAttempts
			return nil
		}
		// A missing or wrong password rolls back: the code stays usable.
		if err := s.confirmPassword(ctx, r, acc.ID, acc.Email, password, meta); err != nil {
			return err
		}

		if err := r.UseEmailCode(ctx, row.ID); err != nil {
			return fmt.Errorf("use code: %w", err)
		}
		// The emailed link would verify the same address; retire it too.
		if err := r.InvalidateEmailTokens(ctx, gen.InvalidateEmailTokensParams{UserID: acc.ID, Purpose: model.PurposeVerify}); err != nil {
			return fmt.Errorf("retire verify link: %w", err)
		}
		if err := r.MarkEmailVerified(ctx, acc.ID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		if err := s.record(ctx, r, &acc.ID, model.EventEmailVerified, "", meta, nil); err != nil {
			return err
		}
		now := s.now()
		if tokens, err = s.startSession(ctx, r, acc.ID, model.ProviderPassword, meta, now); err != nil {
			return err
		}
		if err := r.TouchLastLogin(ctx, acc.ID); err != nil {
			return fmt.Errorf("touch last login: %w", err)
		}
		return s.record(ctx, r, &acc.ID, model.EventLoginSucceeded, model.ProviderPassword, meta,
			map[string]any{"key": s.passwordKey(acc.Email)})
	})
	switch {
	case err != nil:
		return model.Tokens{}, err
	case tokens.AccessToken == "" && tooMany:
		return model.Tokens{}, model.ErrTooManyCodeAttempts
	case tokens.AccessToken == "":
		return model.Tokens{}, model.ErrInvalidCode
	}
	return tokens, nil
}

// ResendVerification emails a new link if the account exists and isn't verified.
// Always returns nil for unknown or already-verified emails.
func (s *Service) ResendVerification(ctx context.Context, rawEmail string, meta model.RequestMeta) error {
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
		if acc.EmailVerifiedAt != nil || acc.Status != "active" {
			return nil
		}
		// Each code gets a few tries, so cap codes per account too: otherwise
		// asking for new ones from many IPs would allow unlimited guesses.
		sent, err := r.CountRecentEmailCodes(ctx, gen.CountRecentEmailCodesParams{
			UserID: acc.ID, Purpose: model.PurposeVerify, Since: s.now().Add(-time.Hour),
		})
		if err != nil {
			return fmt.Errorf("count codes: %w", err)
		}
		if sent >= model.VerifyCodesPerHour {
			return nil // same answer; the latest code still works
		}
		if err := s.sendVerifyLink(ctx, tx, r, acc.ID, acc.Email); err != nil {
			return err
		}
		return s.record(ctx, r, &acc.ID, model.EventVerificationSent, "", meta, nil)
	})
}

// sendVerifyLink issues a verification code and link, retiring earlier ones, and queues the email.
func (s *Service) sendVerifyLink(ctx context.Context, tx pgx.Tx, r repository.Repository, userID uuid.UUID, email string) error {
	raw, err := s.issueEmailToken(ctx, r, userID, email, model.PurposeVerify, model.VerifyTokenTTL)
	if err != nil {
		return err
	}
	if err := r.InvalidateEmailCodes(ctx, gen.InvalidateEmailCodesParams{UserID: userID, Purpose: model.PurposeVerify}); err != nil {
		return fmt.Errorf("invalidate old codes: %w", err)
	}
	code, hash, err := newEmailCode(s.cfg.EmailCodeKey)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("email code id: %w", err)
	}
	if err := r.CreateEmailCode(ctx, gen.CreateEmailCodeParams{
		ID: id, UserID: userID, Purpose: model.PurposeVerify, Email: email, CodeHash: hash, ExpiresAt: s.now().Add(model.VerifyCodeTTL),
	}); err != nil {
		return fmt.Errorf("create email code: %w", err)
	}
	return s.enqueue(ctx, tx, verifyEmail(email, code, s.link("/verify-email", raw)))
}
