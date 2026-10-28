package auth

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
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
	cfg      Config

	dummyHash string // verified against for unknown emails so timing doesn't reveal them
	now       func() time.Time
}

// New builds the service. breach may be nil; providers may be empty.
func New(pool *pgxpool.Pool, tokens TokenIssuer, queue Enqueuer, profiles ProfileCreator, breach BreachChecker, providers Providers, cfg Config) (*Service, error) {
	if len(cfg.EmailCodeKey) != 32 {
		return nil, errors.New("auth: EMAIL_CODE_KEY must be 32 bytes")
	}
	dummy, err := hashPassword("not-a-real-password-Aa1!")
	if err != nil {
		return nil, err
	}
	return &Service{
		pool: pool, tokens: tokens, queue: queue, profiles: profiles, breach: breach, oauth: providers, cfg: cfg,
		dummyHash: dummy,
		now:       func() time.Time { return time.Now().UTC() },
	}, nil
}

// --- Signup and email verification ---

// Signup creates an unverified password account and emails a verification link.
// It returns nil whether or not the email is already registered (no enumeration);
// an existing owner gets a different email instead.
func (s *Service) Signup(ctx context.Context, in SignupInput, meta RequestMeta) error {
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return err
	}
	firstName := strings.TrimSpace(in.FirstName)
	if firstName == "" {
		return ErrInvalidInput.WithDetails(apperr.Detail{Field: "first_name", Message: "Enter your first name"})
	}
	if !in.AcceptTerms {
		return ErrTermsRequired
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
		r := newRepository(tx)

		existing, err := r.GetAccountByEmail(ctx, email)
		switch {
		case err == nil:
			return s.emailExistingAccount(ctx, tx, r, existing)
		case !isNoRows(err):
			return fmt.Errorf("look up email: %w", err)
		}

		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("user id: %w", err)
		}
		if err := r.CreateUser(ctx, gen.CreateUserParams{ID: id, Email: email, PasswordHash: &hash}); err != nil {
			if isUniqueViolation(err) {
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
		return s.record(ctx, r, &id, eventSignup, providerPassword, meta, nil)
	})
}

// emailExistingAccount tells the real owner someone tried to sign up with their email.
func (s *Service) emailExistingAccount(ctx context.Context, tx pgx.Tx, r repository, acc gen.GetAccountByEmailRow) error {
	if acc.EmailVerifiedAt == nil {
		// Unverified account: send a set-password link rather than re-verifying, so
		// a password chosen by whoever created the account is never activated.
		raw, err := s.issueEmailToken(ctx, r, acc.ID, acc.Email, purposeReset, resetTokenTTL)
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
func (s *Service) VerifyEmail(ctx context.Context, token, password string, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.ConsumeEmailToken(ctx, gen.ConsumeEmailTokenParams{TokenHash: hashToken(token), Purpose: purposeVerify})
		if isNoRows(err) {
			return ErrInvalidLink
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
		return s.record(ctx, r, &row.UserID, eventEmailVerified, "", meta, nil)
	})
}

// confirmPassword checks password against the account's own, if it has one.
// Wrong guesses count toward the same limits as login.
func (s *Service) confirmPassword(ctx context.Context, r repository, userID uuid.UUID, email, password string, meta RequestMeta) error {
	stored, err := r.GetPasswordHash(ctx, userID)
	if err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if stored == nil {
		return nil // social-only account: nobody chose a password
	}
	if password == "" {
		return ErrPasswordRequired
	}
	key := s.passwordKey(email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return err
	} else if blocked {
		return ErrTooManyAttempts
	}
	ok, _, err := verifyPassword(password, *stored)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &userID, meta, "wrong_password", key)
		return ErrVerifyWrongPassword
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
func (s *Service) VerifyEmailCode(ctx context.Context, rawEmail, code, password string, meta RequestMeta) (Tokens, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return Tokens{}, err
	}
	code = strings.TrimSpace(code)
	given := hashEmailCode(s.cfg.EmailCodeKey, code)

	var tokens Tokens
	var tooMany bool
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		acc, err := r.GetAccountByEmail(ctx, email)
		if isNoRows(err) {
			return ErrInvalidCode
		}
		if err != nil {
			return fmt.Errorf("look up email: %w", err)
		}
		if acc.EmailVerifiedAt != nil || acc.Status != "active" {
			return ErrInvalidCode
		}
		row, err := r.GetActiveEmailCode(ctx, gen.GetActiveEmailCodeParams{UserID: acc.ID, Purpose: purposeVerify})
		if isNoRows(err) {
			return ErrInvalidCode
		}
		if err != nil {
			return fmt.Errorf("load code: %w", err)
		}
		if row.Attempts >= verifyCodeMaxAttempts {
			return ErrTooManyCodeAttempts
		}
		if !hmac.Equal(row.CodeHash, given) {
			attempts, err := r.RecordEmailCodeAttempt(ctx, row.ID)
			if err != nil {
				return fmt.Errorf("count attempt: %w", err)
			}
			// Commit the attempt count; the answer is decided after the transaction.
			tooMany = attempts >= verifyCodeMaxAttempts
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
		if err := r.InvalidateEmailTokens(ctx, gen.InvalidateEmailTokensParams{UserID: acc.ID, Purpose: purposeVerify}); err != nil {
			return fmt.Errorf("retire verify link: %w", err)
		}
		if err := r.MarkEmailVerified(ctx, acc.ID); err != nil {
			return fmt.Errorf("mark verified: %w", err)
		}
		if err := s.record(ctx, r, &acc.ID, eventEmailVerified, "", meta, nil); err != nil {
			return err
		}
		now := s.now()
		if tokens, err = s.startSession(ctx, r, acc.ID, providerPassword, meta, now); err != nil {
			return err
		}
		if err := r.TouchLastLogin(ctx, acc.ID); err != nil {
			return fmt.Errorf("touch last login: %w", err)
		}
		return s.record(ctx, r, &acc.ID, eventLoginSucceeded, providerPassword, meta,
			map[string]any{"key": s.passwordKey(acc.Email)})
	})
	switch {
	case err != nil:
		return Tokens{}, err
	case tokens.AccessToken == "" && tooMany:
		return Tokens{}, ErrTooManyCodeAttempts
	case tokens.AccessToken == "":
		return Tokens{}, ErrInvalidCode
	}
	return tokens, nil
}

// ResendVerification emails a new link if the account exists and isn't verified.
// Always returns nil for unknown or already-verified emails.
func (s *Service) ResendVerification(ctx context.Context, rawEmail string, meta RequestMeta) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		acc, err := r.GetAccountByEmail(ctx, email)
		if isNoRows(err) {
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
			UserID: acc.ID, Purpose: purposeVerify, Since: s.now().Add(-time.Hour),
		})
		if err != nil {
			return fmt.Errorf("count codes: %w", err)
		}
		if sent >= verifyCodesPerHour {
			return nil // same answer; the latest code still works
		}
		if err := s.sendVerifyLink(ctx, tx, r, acc.ID, acc.Email); err != nil {
			return err
		}
		return s.record(ctx, r, &acc.ID, eventVerificationSent, "", meta, nil)
	})
}

// --- Login, refresh, logout ---

// Login checks email and password and starts a session.
func (s *Service) Login(ctx context.Context, rawEmail, password string, meta RequestMeta) (Tokens, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return Tokens{}, ErrInvalidCredentials
	}
	key := s.passwordKey(email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return Tokens{}, err
	} else if blocked {
		return Tokens{}, ErrTooManyAttempts
	}
	now := s.now()

	r := newRepository(s.pool)
	user, err := r.GetUserForLogin(ctx, email)
	if err != nil && !isNoRows(err) {
		return Tokens{}, fmt.Errorf("look up user: %w", err)
	}
	if err != nil || user.PasswordHash == nil {
		// Unknown email or social-only account: spend the same time, give the same answer.
		_, _, _ = verifyPassword(password, s.dummyHash)
		var uid *uuid.UUID
		if err == nil {
			uid = &user.ID
		}
		s.recordFailedLogin(ctx, uid, meta, "invalid_credentials", key)
		return Tokens{}, ErrInvalidCredentials
	}

	ok, rehash, err := verifyPassword(password, *user.PasswordHash)
	if err != nil {
		return Tokens{}, fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &user.ID, meta, "invalid_credentials", key)
		return Tokens{}, ErrInvalidCredentials
	}
	// Only now, with the right password, is it safe to say why sign-in can't proceed.
	if user.Status != "active" {
		s.recordFailedLogin(ctx, &user.ID, meta, "account_disabled", "")
		return Tokens{}, ErrAccountDisabled
	}
	if user.EmailVerifiedAt == nil {
		s.recordFailedLogin(ctx, &user.ID, meta, "email_not_verified", "")
		return Tokens{}, ErrEmailNotVerified
	}

	var tokens Tokens
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		if rehash {
			if h, err := hashPassword(password); err == nil {
				if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: user.ID, PasswordHash: &h}); err != nil {
					return fmt.Errorf("upgrade password hash: %w", err)
				}
			}
		}
		if tokens, err = s.startSession(ctx, r, user.ID, providerPassword, meta, now); err != nil {
			return err
		}
		if err := r.TouchLastLogin(ctx, user.ID); err != nil {
			return fmt.Errorf("touch last login: %w", err)
		}
		// The key restarts the wrong-password count (see attempts.go).
		return s.record(ctx, r, &user.ID, eventLoginSucceeded, providerPassword, meta, map[string]any{"key": key})
	})
	return tokens, err
}

// Refresh rotates a refresh token and issues a new access token. Presenting a
// token that was already rotated means it was copied: the whole session is
// revoked, unless it was rotated less than refreshReuseGrace ago.
func (s *Service) Refresh(ctx context.Context, rawToken string, meta RequestMeta) (Tokens, error) {
	if rawToken == "" {
		return Tokens{}, ErrInvalidRefresh
	}
	now := s.now()
	var tokens Tokens
	reused := false

	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.GetRefreshTokenForUpdate(ctx, hashToken(rawToken))
		if isNoRows(err) {
			return ErrInvalidRefresh
		}
		if err != nil {
			return fmt.Errorf("load refresh token: %w", err)
		}

		// Inside the grace window a used token gets a sibling instead of a revocation
		// (another tab refreshed with the same cookie a moment ago).
		graced := row.UsedAt != nil && now.Sub(*row.UsedAt) < refreshReuseGrace
		if row.UsedAt != nil && !graced {
			// Commit the revocation, then report failure after the transaction.
			reused = true
			reason := revokeReuseDetected
			if err := r.RevokeSessionByID(ctx, gen.RevokeSessionByIDParams{ID: row.SessionID, Reason: &reason}); err != nil {
				return fmt.Errorf("revoke reused session: %w", err)
			}
			return s.record(ctx, r, &row.UserID, eventRefreshReuseDetected, "", meta,
				map[string]any{"session_id": row.SessionID})
		}
		if row.SessionRevokedAt != nil || !now.Before(row.ExpiresAt) || !now.Before(row.SessionExpiresAt) ||
			row.UserStatus != "active" || row.UserEmailVerifiedAt == nil {
			return ErrInvalidRefresh
		}

		if !graced {
			if err := r.MarkRefreshTokenUsed(ctx, gen.MarkRefreshTokenUsedParams{ID: row.ID, UsedAt: &now}); err != nil {
				return fmt.Errorf("mark refresh token used: %w", err)
			}
		}
		exp := s.sessionExpiry(row.SessionCreatedAt, now)
		if !now.Before(exp) {
			return ErrInvalidRefresh // past SessionMaxAge: sign in again
		}
		refresh, refreshExp, err := s.newRefreshToken(ctx, r, row.SessionID, exp)
		if err != nil {
			return err
		}
		if err := r.ExtendSession(ctx, gen.ExtendSessionParams{ID: row.SessionID, Ip: parseIP(meta.IP), ExpiresAt: refreshExp}); err != nil {
			return fmt.Errorf("extend session: %w", err)
		}
		access, accessExp, err := s.tokens.Issue(row.UserID, row.SessionID, now)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}
		tokens = Tokens{
			Client: row.Client, AccessToken: access, AccessExpiresAt: accessExp,
			RefreshToken: refresh, RefreshExpiresAt: refreshExp,
		}
		return nil
	})
	if err != nil {
		return Tokens{}, err
	}
	if reused {
		return Tokens{}, ErrInvalidRefresh
	}
	return tokens, nil
}

// Logout ends the current session.
func (s *Service) Logout(ctx context.Context, userID, sessionID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeLogout
		if _, err := r.RevokeSession(ctx, gen.RevokeSessionParams{ID: sessionID, UserID: userID, Reason: &reason}); err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		return s.record(ctx, r, &userID, eventLogout, "", meta, nil)
	})
}

// LogoutAll ends every session of the user, including this one.
func (s *Service) LogoutAll(ctx context.Context, userID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeLogoutAll
		n, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: userID, Reason: &reason})
		if err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return s.record(ctx, r, &userID, eventLogoutAll, "", meta, map[string]any{"sessions": n})
	})
}

// --- Passwords ---

// ForgotPassword emails a reset link if the account exists. Always returns nil
// for unknown emails.
func (s *Service) ForgotPassword(ctx context.Context, rawEmail string, meta RequestMeta) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		acc, err := r.GetAccountByEmail(ctx, email)
		if isNoRows(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("look up email: %w", err)
		}
		if acc.Status != "active" {
			return nil
		}
		raw, err := s.issueEmailToken(ctx, r, acc.ID, acc.Email, purposeReset, resetTokenTTL)
		if err != nil {
			return err
		}
		if err := s.enqueue(ctx, tx, resetPasswordEmail(acc.Email, s.link("/reset-password", raw))); err != nil {
			return err
		}
		return s.record(ctx, r, &acc.ID, eventPasswordResetRequested, "", meta, nil)
	})
}

// ResetPassword sets a new password from an emailed link, marks the email
// verified (the link proves ownership) and signs out every device.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string, meta RequestMeta) error {
	hash, err := s.newPasswordHash(ctx, newPassword)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.ConsumeEmailToken(ctx, gen.ConsumeEmailTokenParams{TokenHash: hashToken(token), Purpose: purposeReset})
		if isNoRows(err) {
			return ErrInvalidLink
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
				if err := s.record(ctx, r, &row.UserID, eventIdentityUnlinked, "", meta,
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
		reason := revokePasswordReset
		if _, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: row.UserID, Reason: &reason}); err != nil {
			return fmt.Errorf("revoke sessions: %w", err)
		}
		return s.record(ctx, r, &row.UserID, eventPasswordResetCompleted, "", meta, nil)
	})
}

// ChangePassword replaces the password after checking the current one, and
// signs out every other device.
func (s *Service) ChangePassword(ctx context.Context, userID, sessionID uuid.UUID, current, next string, meta RequestMeta) error {
	r := newRepository(s.pool)
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	stored, err := r.GetPasswordHash(ctx, userID)
	if err != nil {
		return fmt.Errorf("load password: %w", err)
	}
	if stored == nil {
		return ErrNoPassword
	}
	// A stolen session must not be a way to guess the password without limits.
	key := s.passwordKey(acc.Email)
	if blocked, err := s.passwordBlocked(ctx, key, meta); err != nil {
		return err
	} else if blocked {
		return ErrTooManyAttempts
	}
	ok, _, err := verifyPassword(current, *stored)
	if err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !ok {
		s.recordFailedLogin(ctx, &userID, meta, "wrong_current_password", key)
		return ErrWrongPassword.WithDetails(apperr.Detail{Field: "current_password", Message: "Current password is incorrect"})
	}
	hash, err := s.newPasswordHash(ctx, next)
	if err != nil {
		return err
	}

	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: userID, PasswordHash: &hash}); err != nil {
			return fmt.Errorf("set password: %w", err)
		}
		reason := revokePasswordChanged
		if _, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: userID, Reason: &reason, KeepID: &sessionID}); err != nil {
			return fmt.Errorf("revoke other sessions: %w", err)
		}
		return s.record(ctx, r, &userID, eventPasswordChanged, "", meta, nil)
	})
}

// --- Sessions and events ---

// ListSessions returns the user's active sessions, this device first.
func (s *Service) ListSessions(ctx context.Context, userID, currentSessionID uuid.UUID) ([]Session, error) {
	rows, err := newRepository(s.pool).ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, Session{
			ID: row.ID, Provider: row.Provider, Client: row.Client,
			DeviceName: row.DeviceName, DeviceType: row.DeviceType, LastIP: ipString(row.LastIp),
			LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt, Current: row.ID == currentSessionID,
		})
	}
	// Rows arrive most recent first; move the current device to the top, keep the rest in order.
	slices.SortStableFunc(out, func(a, b Session) int {
		switch {
		case a.Current == b.Current:
			return 0
		case a.Current:
			return -1
		default:
			return 1
		}
	})
	return out, nil
}

// RevokeSession signs out one of the user's own sessions.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		reason := revokeByUser
		n, err := r.RevokeSession(ctx, gen.RevokeSessionParams{ID: sessionID, UserID: userID, Reason: &reason})
		if err != nil {
			return fmt.Errorf("revoke session: %w", err)
		}
		if n == 0 {
			return ErrSessionNotFound
		}
		return s.record(ctx, r, &userID, eventSessionRevoked, "", meta, map[string]any{"session_id": sessionID})
	})
}

// ListEvents returns the user's 50 most recent security events.
func (s *Service) ListEvents(ctx context.Context, userID uuid.UUID) ([]Event, error) {
	rows, err := newRepository(s.pool).ListAuthEvents(ctx, &userID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		device, _ := describeDevice(clientWeb, row.UserAgent, "")
		e := Event{ID: row.ID, Event: row.Event, IP: ipString(row.Ip), Device: device, CreatedAt: row.CreatedAt}
		if row.Provider != nil {
			e.Provider = *row.Provider
		}
		out = append(out, e)
	}
	return out, nil
}

// --- helpers ---

// newPasswordHash checks the rules and the breach list, then hashes.
func (s *Service) newPasswordHash(ctx context.Context, pw string) (string, error) {
	if d := checkPasswordRules(pw); d != nil {
		return "", ErrWeakPassword.WithDetails(d...)
	}
	if s.breach != nil {
		breached, err := s.breach.IsBreached(ctx, pw)
		if err != nil {
			// Fail open: the breach service being down must not block signups.
			logger.FromContext(ctx).Warn("password breach check unavailable", "err", err)
		}
		if breached {
			return "", ErrBreachedPassword.WithDetails(apperr.Detail{Field: "password", Message: ErrBreachedPassword.Message})
		}
	}
	return hashPassword(pw)
}

func (s *Service) startSession(ctx context.Context, r repository, userID uuid.UUID, provider string, meta RequestMeta, now time.Time) (Tokens, error) {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return Tokens{}, fmt.Errorf("session id: %w", err)
	}
	client := meta.Client
	if client != clientMobile {
		client = clientWeb
	}
	name, kind := describeDevice(client, meta.UserAgent, meta.DeviceHeader)

	if err := r.CreateSession(ctx, gen.CreateSessionParams{
		ID: sessionID, UserID: userID, Provider: provider, Client: client,
		UserAgent: truncate(meta.UserAgent, 512), DeviceName: name, DeviceType: kind,
		Ip: parseIP(meta.IP), Now: now, ExpiresAt: s.sessionExpiry(now, now),
	}); err != nil {
		return Tokens{}, fmt.Errorf("create session: %w", err)
	}
	refresh, refreshExp, err := s.newRefreshToken(ctx, r, sessionID, s.sessionExpiry(now, now))
	if err != nil {
		return Tokens{}, err
	}
	access, accessExp, err := s.tokens.Issue(userID, sessionID, now)
	if err != nil {
		return Tokens{}, fmt.Errorf("issue access token: %w", err)
	}
	return Tokens{
		Client: client, AccessToken: access, AccessExpiresAt: accessExp,
		RefreshToken: refresh, RefreshExpiresAt: refreshExp,
	}, nil
}

// sessionExpiry is when a session started at created ends if refreshed at now:
// RefreshTTL after the last refresh, but never past SessionMaxAge.
func (s *Service) sessionExpiry(created, now time.Time) time.Time {
	exp := now.Add(s.cfg.RefreshTTL)
	if s.cfg.SessionMaxAge > 0 {
		if limit := created.Add(s.cfg.SessionMaxAge); limit.Before(exp) {
			return limit
		}
	}
	return exp
}

func (s *Service) newRefreshToken(ctx context.Context, r repository, sessionID uuid.UUID, exp time.Time) (string, time.Time, error) {
	raw, hash, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("refresh token id: %w", err)
	}
	if err := r.CreateRefreshToken(ctx, gen.CreateRefreshTokenParams{ID: id, SessionID: sessionID, TokenHash: hash, ExpiresAt: exp}); err != nil {
		return "", time.Time{}, fmt.Errorf("create refresh token: %w", err)
	}
	return raw, exp, nil
}

// issueEmailToken invalidates earlier unused links of the same purpose and creates a new one.
func (s *Service) issueEmailToken(ctx context.Context, r repository, userID uuid.UUID, email, purpose string, ttl time.Duration) (string, error) {
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

// sendVerifyLink issues a verification code and link, retiring earlier ones, and queues the email.
func (s *Service) sendVerifyLink(ctx context.Context, tx pgx.Tx, r repository, userID uuid.UUID, email string) error {
	raw, err := s.issueEmailToken(ctx, r, userID, email, purposeVerify, verifyTokenTTL)
	if err != nil {
		return err
	}
	if err := r.InvalidateEmailCodes(ctx, gen.InvalidateEmailCodesParams{UserID: userID, Purpose: purposeVerify}); err != nil {
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
		ID: id, UserID: userID, Purpose: purposeVerify, Email: email, CodeHash: hash, ExpiresAt: s.now().Add(verifyCodeTTL),
	}); err != nil {
		return fmt.Errorf("create email code: %w", err)
	}
	return s.enqueue(ctx, tx, verifyEmail(email, code, s.link("/verify-email", raw)))
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
func (s *Service) record(ctx context.Context, r repository, userID *uuid.UUID, event, provider string, meta RequestMeta, metadata map[string]any) error {
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
		Ip: parseIP(meta.IP), UserAgent: truncate(meta.UserAgent, 512), Metadata: md,
	}); err != nil {
		return fmt.Errorf("record %s: %w", event, err)
	}
	return nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.TrimSpace(raw)
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || len(email) > 254 {
		return "", ErrInvalidInput.WithDetails(apperr.Detail{Field: "email", Message: "Enter a valid email address"})
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
	return ErrInvalidInput.WithDetails(apperr.Detail{Field: "timezone", Message: "Unknown timezone"})
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
