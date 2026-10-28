package auth

import (
	"context"
	"crypto/hmac"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/oauth2"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Social sign-in. The flow, for both web and mobile:
//
//  1. GET /oauth/{provider}/start stores an oauth_flows row (state hash, PKCE
//     verifier, nonce), sets the relay_oauth cookie to the state, and redirects
//     to the provider.
//  2. The provider redirects back to /oauth/{provider}/callback. The state must
//     match the cookie (so a stolen callback URL can't sign a victim into the
//     attacker's account), the row is consumed, the code is exchanged, and the
//     linking rules below pick the user.
//  3. Web: auth cookies are set and the browser goes to redirect_to. Mobile: the
//     browser goes to the app's redirect_to with a one-time code, which the app
//     redeems at POST /oauth/token with its own PKCE verifier.
//
// A new sign-in whose provider doesn't vouch for the email (Facebook, or one
// that gives no email at all) ends in "awaiting_email": the client asks for an
// address (prefilled with the provider's), POST /oauth/complete emails it a code,
// and POST /oauth/complete/verify creates or links the account once the code
// matches. Nobody can attach a provider to an address they can't read.

// Callback error codes that aren't apperr errors.
const (
	oauthErrCancelled = "oauth_cancelled"
	oauthErrServer    = "server_error"
)

var (
	// errNeedEmail: the provider gave no usable email; ask the user for one.
	errNeedEmail = errors.New("provider gave no verified email")
	// pkceChallenge is an S256 code challenge: 43 base64url characters.
	pkceChallenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	// ErrVerifyEmailFirst blocks setting a password until the email is proven.
	ErrVerifyEmailFirst = apperr.Forbidden("email_not_verified", "Verify your email address before setting a password")
	// ErrInvalidSignedRequest is a Meta callback that fails signature checks.
	ErrInvalidSignedRequest = apperr.Validation("invalid_signed_request", "The signed request is invalid")
)

// ProviderNames lists the social sign-in providers this server offers.
func (s *Service) ProviderNames() []string {
	if s == nil {
		return nil
	}
	return s.oauth.Names()
}

// --- Start ---

// StartOAuth records a new sign-in attempt and returns the provider URL to
// send the browser to, plus the state to bind to the browser in a cookie.
func (s *Service) StartOAuth(ctx context.Context, in OAuthStart) (authURL, state string, err error) {
	p, ok := s.oauth[in.Provider]
	if !ok {
		return "", "", ErrProviderNotAvailable
	}
	client := in.Client
	if client != clientMobile {
		client = clientWeb
	}
	redirect, err := s.checkRedirect(in.RedirectTo, client)
	if err != nil {
		return "", "", err
	}
	tz, err := normalizeTimezone(in.Timezone)
	if err != nil {
		tz = "UTC" // a bad timezone must not block sign-in; the user can fix it in settings
	}
	var challenge *string
	if client == clientMobile {
		if !pkceChallenge.MatchString(in.AppCodeChallenge) {
			return "", "", ErrInvalidInput.WithDetails(apperr.Detail{
				Field: "code_challenge", Message: "Mobile sign-in needs a PKCE S256 code_challenge",
			})
		}
		challenge = &in.AppCodeChallenge
	}

	r := newRepository(s.pool)
	var linkUser *uuid.UUID
	intent := intentLogin
	if in.Intent == intentLink {
		// Connecting an account is sensitive: a stolen session must not add the thief's login.
		if err := s.requireRecentSignIn(ctx, r, in.LinkUserID, in.SessionID); err != nil {
			return "", "", err
		}
		intent, linkUser = intentLink, &in.LinkUserID
	}

	state, stateHash, err := newToken()
	if err != nil {
		return "", "", err
	}
	nonce, _, err := newToken()
	if err != nil {
		return "", "", err
	}
	verifier := oauth2.GenerateVerifier()
	id, err := uuid.NewV7()
	if err != nil {
		return "", "", fmt.Errorf("flow id: %w", err)
	}
	if err := r.CreateOAuthFlow(ctx, gen.CreateOAuthFlowParams{
		ID: id, StateHash: stateHash, Provider: p.Name(), Intent: intent, LinkUserID: linkUser,
		CodeVerifier: verifier, Nonce: nonce, RedirectTo: redirect, Client: client, Timezone: tz,
		AppCodeChallenge: challenge, ExpiresAt: s.now().Add(oauthFlowTTL),
	}); err != nil {
		return "", "", fmt.Errorf("create oauth flow: %w", err)
	}
	return p.AuthURL(state, oauth2.S256ChallengeFromVerifier(verifier), nonce, s.callbackURL(p.Name())), state, nil
}

// --- Callback ---

// CompleteOAuth handles the provider's callback. It never returns an error:
// every outcome, failures included, is a redirect the client can show.
func (s *Service) CompleteOAuth(ctx context.Context, provider, code, state, cookieState, providerError string, meta RequestMeta) OAuthResult {
	fallback := s.defaultRedirect()
	p, ok := s.oauth[provider]
	if !ok {
		return OAuthResult{RedirectTo: fallback, ErrorCode: ErrProviderNotAvailable.Code}
	}
	if state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookieState)) != 1 {
		return OAuthResult{RedirectTo: fallback, ErrorCode: ErrInvalidOAuthState.Code}
	}
	flow, err := newRepository(s.pool).ConsumeOAuthFlow(ctx, gen.ConsumeOAuthFlowParams{StateHash: hashToken(state), Provider: provider})
	if isNoRows(err) {
		return OAuthResult{RedirectTo: fallback, ErrorCode: ErrInvalidOAuthState.Code}
	}
	if err != nil {
		return s.oauthFailure(ctx, fallback, fmt.Errorf("consume oauth flow: %w", err))
	}
	if providerError != "" {
		code := ErrOAuthFailed.Code
		if providerError == "access_denied" {
			code = oauthErrCancelled
		}
		return OAuthResult{RedirectTo: flow.RedirectTo, ErrorCode: code}
	}

	id, err := p.Exchange(ctx, code, flow.CodeVerifier, flow.Nonce, s.callbackURL(provider))
	if err != nil {
		return s.oauthFailure(ctx, flow.RedirectTo, err)
	}
	if flow.Intent == intentLink && flow.LinkUserID != nil {
		return s.linkIdentity(ctx, id, *flow.LinkUserID, flow.RedirectTo, meta)
	}
	return s.signInWithIdentity(ctx, id, flow, meta)
}

func (s *Service) signInWithIdentity(ctx context.Context, id ProviderIdentity, flow gen.ConsumeOAuthFlowRow, meta RequestMeta) OAuthResult {
	res := OAuthResult{RedirectTo: flow.RedirectTo}
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		userID, err := s.resolveIdentity(ctx, tx, r, id, flow.Timezone, meta)
		if errors.Is(err, errNeedEmail) {
			pending, err := json.Marshal(id)
			if err != nil {
				return fmt.Errorf("pending identity: %w", err)
			}
			res.CompleteToken, err = s.createPendingFlow(ctx, r, flowStatusAwaitingEmail, id.Provider, flow.Client,
				flow.RedirectTo, flow.Timezone, flow.AppCodeChallenge, pending, nil, oauthAwaitingTTL)
			if e := providerEmail(id); e != nil {
				res.PendingEmail = *e // prefills the form; the person still proves it
			}
			return err
		}
		if err != nil {
			return err
		}
		if err := checkActive(ctx, r, userID); err != nil {
			return err
		}
		if flow.Client == clientMobile {
			// The session is created when the app redeems the code, with the app's own device details.
			res.AppCode, err = s.createPendingFlow(ctx, r, flowStatusReady, id.Provider, flow.Client,
				flow.RedirectTo, flow.Timezone, flow.AppCodeChallenge, nil, &userID, oauthReadyTTL)
			return err
		}
		meta.Client = clientWeb
		t, err := s.signIn(ctx, r, userID, id.Provider, meta)
		res.Tokens = &t
		return err
	})
	if err != nil {
		return s.oauthFailure(ctx, flow.RedirectTo, err)
	}
	return res
}

// resolveIdentity applies the linking rules (docs/PHASE-2-AUTH.md, section 4)
// and returns the user this provider identity signs in as.
func (s *Service) resolveIdentity(ctx context.Context, tx pgx.Tx, r repository, id ProviderIdentity, tz string, meta RequestMeta) (uuid.UUID, error) {
	existing, err := r.GetIdentity(ctx, gen.GetIdentityParams{Provider: id.Provider, ProviderUserID: id.Subject})
	switch {
	case err == nil:
		// Known identity: refresh what the provider reports. The profile is never overwritten.
		if err := r.TouchIdentity(ctx, gen.TouchIdentityParams{
			ID: existing.ID, Email: providerEmail(id), EmailVerified: id.EmailVerified,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("touch identity: %w", err)
		}
		return existing.UserID, nil
	case !isNoRows(err):
		return uuid.Nil, fmt.Errorf("look up identity: %w", err)
	}

	email, err := normalizeEmail(id.Email)
	if err != nil || !id.EmailVerified {
		// Never create or attach an account on an email nobody has proven.
		return uuid.Nil, errNeedEmail
	}
	id.Email = email

	acc, err := r.GetAccountByEmail(ctx, email)
	switch {
	case err == nil:
		if acc.EmailVerifiedAt == nil {
			// The provider proves this person owns the email, so whoever created the
			// unverified account didn't: their password and sessions go.
			if err := s.claimUnverifiedAccount(ctx, r, acc.ID, meta); err != nil {
				return uuid.Nil, err
			}
		}
		if err := s.addIdentity(ctx, r, acc.ID, id, meta); err != nil {
			return uuid.Nil, err
		}
		return acc.ID, nil
	case !isNoRows(err):
		return uuid.Nil, fmt.Errorf("look up email: %w", err)
	}
	return s.createSocialUser(ctx, tx, r, id, tz, meta)
}

func (s *Service) claimUnverifiedAccount(ctx context.Context, r repository, userID uuid.UUID, meta RequestMeta) error {
	if err := r.MarkEmailVerified(ctx, userID); err != nil {
		return fmt.Errorf("mark verified: %w", err)
	}
	if err := r.SetPasswordHash(ctx, gen.SetPasswordHashParams{ID: userID, PasswordHash: nil}); err != nil {
		return fmt.Errorf("clear password: %w", err)
	}
	reason := revokeAccountClaimed
	if _, err := r.RevokeUserSessions(ctx, gen.RevokeUserSessionsParams{UserID: userID, Reason: &reason}); err != nil {
		return fmt.Errorf("revoke sessions: %w", err)
	}
	// A social login whose provider never vouched for the email (Facebook) was
	// attached by the same unproven creator: it must not keep access either.
	n, err := r.DeleteUnverifiedIdentities(ctx, userID)
	if err != nil {
		return fmt.Errorf("remove unverified identities: %w", err)
	}
	if n > 0 {
		if err := s.record(ctx, r, &userID, eventIdentityUnlinked, "", meta,
			map[string]any{"reason": "claimed_by_provider"}); err != nil {
			return err
		}
	}
	return s.record(ctx, r, &userID, eventEmailVerified, "", meta, map[string]any{"reason": "claimed_by_provider"})
}

// createSocialUser creates the user, identity, profile and preferences. The
// email is always proven by now (by the provider or by an emailed code).
func (s *Service) createSocialUser(ctx context.Context, tx pgx.Tx, r repository, id ProviderIdentity, tz string, meta RequestMeta) (uuid.UUID, error) {
	userID, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("user id: %w", err)
	}
	now := s.now()
	if err := r.CreateSocialUser(ctx, gen.CreateSocialUserParams{ID: userID, Email: id.Email, EmailVerifiedAt: &now}); err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, ErrEmailInUse // a concurrent signup took the email
		}
		return uuid.Nil, fmt.Errorf("create user: %w", err)
	}
	if err := s.addIdentity(ctx, r, userID, id, meta); err != nil {
		return uuid.Nil, err
	}
	if err := s.profiles.CreateForUser(ctx, tx, userID, firstNameFor(id), nonEmpty(id.LastName), nonEmpty(id.AvatarURL), tz); err != nil {
		return uuid.Nil, fmt.Errorf("create profile: %w", err)
	}
	return userID, s.record(ctx, r, &userID, eventSignup, id.Provider, meta, nil)
}

func (s *Service) addIdentity(ctx context.Context, r repository, userID uuid.UUID, id ProviderIdentity, meta RequestMeta) error {
	identityID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("identity id: %w", err)
	}
	if err := r.CreateIdentity(ctx, gen.CreateIdentityParams{
		ID: identityID, UserID: userID, Provider: id.Provider, ProviderUserID: id.Subject,
		Email: providerEmail(id), EmailVerified: id.EmailVerified,
	}); err != nil {
		if isUniqueViolation(err) {
			return ErrProviderAlreadyLinked
		}
		return fmt.Errorf("create identity: %w", err)
	}
	return s.record(ctx, r, &userID, eventIdentityLinked, id.Provider, meta, nil)
}

// linkIdentity connects a provider to the signed-in user (intent=link from settings).
func (s *Service) linkIdentity(ctx context.Context, id ProviderIdentity, userID uuid.UUID, redirect string, meta RequestMeta) OAuthResult {
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		existing, err := r.GetIdentity(ctx, gen.GetIdentityParams{Provider: id.Provider, ProviderUserID: id.Subject})
		switch {
		case err == nil && existing.UserID != userID:
			return ErrIdentityInUse
		case err == nil:
			return nil // already connected to this user
		case !isNoRows(err):
			return fmt.Errorf("look up identity: %w", err)
		}
		return s.addIdentity(ctx, r, userID, id, meta)
	})
	if err != nil {
		return s.oauthFailure(ctx, redirect, err)
	}
	return OAuthResult{RedirectTo: redirect, Linked: id.Provider}
}

// --- Second stage: add an email, or redeem a mobile code ---

// SendOAuthEmailCode is the first half of the "add your email" step: it emails
// a code to the address the person typed. It answers the same whether or not
// the address has an account. Mobile flows must present their PKCE verifier.
func (s *Service) SendOAuthEmailCode(ctx context.Context, token, rawEmail, codeVerifier string) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		flow, err := r.GetAwaitingOAuthFlow(ctx, hashToken(token))
		if isNoRows(err) {
			return ErrInvalidOAuthState
		}
		if err != nil {
			return fmt.Errorf("load pending flow: %w", err)
		}
		if flow.Client == clientMobile && !challengeMatches(flow.AppCodeChallenge, codeVerifier) {
			return ErrInvalidAppCode
		}
		if flow.EmailCodeSends >= oauthCodeMaxSends {
			return ErrTooManyCodeSends
		}
		code, hash, err := newEmailCode(s.cfg.EmailCodeKey)
		if err != nil {
			return err
		}
		exp := s.now().Add(verifyCodeTTL)
		if err := r.SetOAuthFlowEmailCode(ctx, gen.SetOAuthFlowEmailCodeParams{
			ID: flow.ID, PendingEmail: &email, EmailCodeHash: hash, EmailCodeExpiresAt: &exp,
		}); err != nil {
			return fmt.Errorf("store code: %w", err)
		}
		return s.enqueue(ctx, tx, confirmEmailCode(email, code))
	})
}

// CompleteOAuthSignup is the second half: with the right code the email is
// proven, so the sign-in creates the account (or links an existing one, as a
// provider-verified email would) and signs in. After verifyCodeMaxAttempts
// wrong codes the flow is gone and the sign-in must start again.
func (s *Service) CompleteOAuthSignup(ctx context.Context, token, code, codeVerifier string, meta RequestMeta) (Tokens, error) {
	given := hashEmailCode(s.cfg.EmailCodeKey, strings.TrimSpace(code))
	var (
		tokens  Tokens
		tooMany bool
	)
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.GetOAuthFlowEmailCode(ctx, hashToken(token))
		if isNoRows(err) {
			return ErrInvalidOAuthState
		}
		if err != nil {
			return fmt.Errorf("load pending flow: %w", err)
		}
		if row.Client == clientMobile && !challengeMatches(row.AppCodeChallenge, codeVerifier) {
			return ErrInvalidAppCode
		}
		if row.PendingEmail == nil || row.EmailCodeExpiresAt == nil || !s.now().Before(*row.EmailCodeExpiresAt) {
			return ErrInvalidCode
		}
		if !hmac.Equal(row.EmailCodeHash, given) {
			attempts, err := r.RecordOAuthFlowCodeAttempt(ctx, row.ID)
			if err != nil {
				return fmt.Errorf("count attempt: %w", err)
			}
			if attempts >= verifyCodeMaxAttempts {
				// The flow is spent: guessing must start over at the provider.
				if err := r.DeleteOAuthFlow(ctx, row.ID); err != nil {
					return fmt.Errorf("end flow: %w", err)
				}
				tooMany = true
			}
			return nil // commit the count; the answer is decided after
		}
		if err := r.DeleteOAuthFlow(ctx, row.ID); err != nil {
			return fmt.Errorf("consume flow: %w", err)
		}

		var id ProviderIdentity
		if err := json.Unmarshal(row.PendingIdentity, &id); err != nil {
			return fmt.Errorf("pending identity: %w", err)
		}
		id.Email, id.EmailVerified = *row.PendingEmail, true // the code proved it
		userID, err := s.resolveIdentity(ctx, tx, r, id, row.Timezone, meta)
		if err != nil {
			return err
		}
		if err := checkActive(ctx, r, userID); err != nil {
			return err
		}
		meta.Client = row.Client
		tokens, err = s.signIn(ctx, r, userID, id.Provider, meta)
		return err
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

// RedeemAppCode turns a mobile one-time code into tokens. The code is spent
// even when the verifier is wrong, so it can't be guessed at.
func (s *Service) RedeemAppCode(ctx context.Context, code, codeVerifier string, meta RequestMeta) (Tokens, error) {
	var (
		tokens Tokens
		bad    bool
	)
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		row, err := r.ConsumePendingOAuthFlow(ctx, gen.ConsumePendingOAuthFlowParams{
			CompletionTokenHash: hashToken(code), Status: flowStatusReady,
		})
		if isNoRows(err) {
			return ErrInvalidAppCode
		}
		if err != nil {
			return fmt.Errorf("consume ready flow: %w", err)
		}
		if row.UserID == nil || !challengeMatches(row.AppCodeChallenge, codeVerifier) {
			bad = true // commit the deletion, then fail
			return nil
		}
		if err := checkActive(ctx, r, *row.UserID); err != nil {
			return err
		}
		meta.Client = clientMobile
		tokens, err = s.signIn(ctx, r, *row.UserID, row.Provider, meta)
		return err
	})
	if err != nil {
		return Tokens{}, err
	}
	if bad {
		return Tokens{}, ErrInvalidAppCode
	}
	return tokens, nil
}

// --- Settings: identities and password ---

// ListIdentities returns the connected providers and whether a password is set.
func (s *Service) ListIdentities(ctx context.Context, userID uuid.UUID) (hasPassword bool, out []Identity, err error) {
	r := newRepository(s.pool)
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return false, nil, fmt.Errorf("load account: %w", err)
	}
	rows, err := r.ListIdentities(ctx, userID)
	if err != nil {
		return false, nil, fmt.Errorf("list identities: %w", err)
	}
	out = make([]Identity, 0, len(rows))
	for _, row := range rows {
		i := Identity{Provider: row.Provider, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt}
		if row.Email != nil {
			i.Email = *row.Email
		}
		out = append(out, i)
	}
	return acc.HasPassword, out, nil
}

// UnlinkIdentity disconnects a provider. It needs a recent sign-in and refuses
// to remove the last way to sign in.
func (s *Service) UnlinkIdentity(ctx context.Context, userID, sessionID uuid.UUID, provider string, meta RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		if err := s.requireRecentSignIn(ctx, r, userID, sessionID); err != nil {
			return err
		}
		n, err := r.DeleteIdentity(ctx, gen.DeleteIdentityParams{UserID: userID, Provider: provider})
		if err != nil {
			return fmt.Errorf("delete identity: %w", err)
		}
		if n == 0 {
			return ErrIdentityNotFound
		}
		acc, err := r.GetAccount(ctx, userID)
		if err != nil {
			return fmt.Errorf("load account: %w", err)
		}
		left, err := r.CountIdentities(ctx, userID)
		if err != nil {
			return fmt.Errorf("count identities: %w", err)
		}
		if !acc.HasPassword && left == 0 {
			return ErrLastSignInMethod // rolls the delete back
		}
		return s.record(ctx, r, &userID, eventIdentityUnlinked, provider, meta, nil)
	})
}

// SetPassword adds a password to an account that signs in only with providers.
// It needs a recent sign-in and a verified email.
func (s *Service) SetPassword(ctx context.Context, userID, sessionID uuid.UUID, password string, meta RequestMeta) error {
	hash, err := s.newPasswordHash(ctx, password)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
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
			return ErrPasswordAlreadySet
		}
		return s.record(ctx, r, &userID, eventPasswordSet, "", meta, nil)
	})
}

// --- Meta data deletion ---

// FacebookDataDeletion handles Meta's data-deletion callback: it verifies the
// signed request, disconnects that Facebook identity and returns the status
// page URL and confirmation code Meta shows the person.
func (s *Service) FacebookDataDeletion(ctx context.Context, signedRequest string, meta RequestMeta) (statusURL, code string, err error) {
	fb, ok := s.oauth[providerFacebook].(*facebook)
	if !ok {
		return "", "", ErrProviderNotAvailable
	}
	fbUserID, err := parseSignedRequest(signedRequest, fb.secret)
	if err != nil {
		return "", "", ErrInvalidSignedRequest.Wrap(err)
	}
	code = uuid.NewString()
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		userID, err := r.DeleteIdentityByProviderUser(ctx, gen.DeleteIdentityByProviderUserParams{
			Provider: providerFacebook, ProviderUserID: fbUserID,
		})
		if isNoRows(err) {
			return nil // nothing stored for this person; still confirm
		}
		if err != nil {
			return fmt.Errorf("delete facebook identity: %w", err)
		}
		return s.record(ctx, r, &userID, eventIdentityUnlinked, providerFacebook, meta,
			map[string]any{"reason": "meta_data_deletion", "confirmation_code": code})
	})
	if err != nil {
		return "", "", err
	}
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + "/data-deletion?code=" + code, code, nil
}

// --- helpers ---

// signIn starts a session and records the sign-in (shared by web, mobile and the email step).
func (s *Service) signIn(ctx context.Context, r repository, userID uuid.UUID, provider string, meta RequestMeta) (Tokens, error) {
	t, err := s.startSession(ctx, r, userID, provider, meta, s.now())
	if err != nil {
		return Tokens{}, err
	}
	if err := r.TouchLastLogin(ctx, userID); err != nil {
		return Tokens{}, fmt.Errorf("touch last login: %w", err)
	}
	return t, s.record(ctx, r, &userID, eventLoginSucceeded, provider, meta, nil)
}

func (s *Service) createPendingFlow(ctx context.Context, r repository, status, provider, client, redirect, tz string,
	challenge *string, pending []byte, userID *uuid.UUID, ttl time.Duration,
) (string, error) {
	_, stateHash, err := newToken() // unused: the row is found by its completion token
	if err != nil {
		return "", err
	}
	raw, hash, err := newToken()
	if err != nil {
		return "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("flow id: %w", err)
	}
	if err := r.CreatePendingOAuthFlow(ctx, gen.CreatePendingOAuthFlowParams{
		ID: id, StateHash: stateHash, Provider: provider, RedirectTo: redirect, Client: client, Timezone: tz,
		AppCodeChallenge: challenge, Status: status, PendingIdentity: pending, CompletionTokenHash: hash,
		UserID: userID, ExpiresAt: s.now().Add(ttl),
	}); err != nil {
		return "", fmt.Errorf("create %s flow: %w", status, err)
	}
	return raw, nil
}

// requireRecentSignIn checks this session proved who it is within ReauthMaxAge.
func (s *Service) requireRecentSignIn(ctx context.Context, r repository, userID, sessionID uuid.UUID) error {
	at, err := r.GetSessionAuthenticatedAt(ctx, gen.GetSessionAuthenticatedAtParams{ID: sessionID, UserID: userID})
	if isNoRows(err) {
		return ErrReauthRequired
	}
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if s.now().Sub(at) > s.cfg.ReauthMaxAge {
		return ErrReauthRequired
	}
	return nil
}

// checkActive refuses a disabled account, and one whose email was never proven
// (sessions need both; see IsSessionActive).
func checkActive(ctx context.Context, r repository, userID uuid.UUID) error {
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	if acc.Status != "active" {
		return ErrAccountDisabled
	}
	if acc.EmailVerifiedAt == nil {
		return ErrEmailNotVerified
	}
	return nil
}

// oauthFailure turns an error into a redirect. Our own failures are logged;
// the client only sees a code.
func (s *Service) oauthFailure(ctx context.Context, redirect string, err error) OAuthResult {
	var ae *apperr.Error
	switch {
	case errors.Is(err, errProviderRejected):
		logger.FromContext(ctx).Warn("oauth sign-in rejected", "err", err)
		return OAuthResult{RedirectTo: redirect, ErrorCode: ErrOAuthFailed.Code}
	case errors.As(err, &ae) && ae.Kind != apperr.KindInternal:
		return OAuthResult{RedirectTo: redirect, ErrorCode: ae.Code}
	default:
		logger.FromContext(ctx).Error("oauth sign-in failed", "err", err)
		return OAuthResult{RedirectTo: redirect, ErrorCode: oauthErrServer}
	}
}

func (s *Service) callbackURL(provider string) string {
	return strings.TrimRight(s.cfg.APIBaseURL, "/") + "/v1/auth/oauth/" + provider + "/callback"
}

func (s *Service) defaultRedirect() string {
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + "/login"
}

// checkRedirect allows any page of the web app and the exact URLs in
// AUTH_ALLOWED_REDIRECTS. The web falls back to the app; mobile must be exact.
func (s *Service) checkRedirect(raw, client string) (string, error) {
	if raw != "" && slices.Contains(s.cfg.AllowedRedirects, raw) {
		return raw, nil
	}
	if client == clientMobile {
		return "", ErrInvalidRedirect
	}
	app, err := url.Parse(s.cfg.AppBaseURL)
	if err == nil && raw != "" {
		if u, err := url.Parse(raw); err == nil && u.Scheme == app.Scheme && u.Host == app.Host && u.User == nil {
			return raw, nil
		}
	}
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + "/", nil
}

func challengeMatches(challenge *string, verifier string) bool {
	if challenge == nil || verifier == "" {
		return false
	}
	got := oauth2.S256ChallengeFromVerifier(verifier)
	return subtle.ConstantTimeCompare([]byte(got), []byte(*challenge)) == 1
}

func providerEmail(id ProviderIdentity) *string {
	if e, err := normalizeEmail(id.Email); err == nil {
		return &e
	}
	return nil
}

func firstNameFor(id ProviderIdentity) string {
	if n := strings.TrimSpace(id.FirstName); n != "" {
		return truncate(n, 100)
	}
	if local, _, ok := strings.Cut(id.Email, "@"); ok && local != "" {
		return truncate(local, 100)
	}
	return "Relay user"
}

func nonEmpty(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// URL is RedirectTo with the outcome added as query parameters.
func (r OAuthResult) URL() string {
	u, err := url.Parse(r.RedirectTo)
	if err != nil {
		return r.RedirectTo
	}
	q := u.Query()
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("error", r.ErrorCode)
	set("linked", r.Linked)
	set("code", r.AppCode)
	set("complete_token", r.CompleteToken)
	set("email", r.PendingEmail)
	u.RawQuery = q.Encode()
	return u.String()
}
