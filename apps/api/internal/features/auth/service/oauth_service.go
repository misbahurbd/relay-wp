package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/repository"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
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
func (s *Service) StartOAuth(ctx context.Context, in model.OAuthStart) (authURL, state string, err error) {
	p, ok := s.oauth[in.Provider]
	if !ok {
		return "", "", model.ErrProviderNotAvailable
	}
	client := in.Client
	if client != model.ClientMobile {
		client = model.ClientWeb
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
	if client == model.ClientMobile {
		if !pkceChallenge.MatchString(in.AppCodeChallenge) {
			return "", "", model.ErrInvalidInput.WithDetails(apperr.Detail{
				Field: "code_challenge", Message: "Mobile sign-in needs a PKCE S256 code_challenge",
			})
		}
		challenge = &in.AppCodeChallenge
	}

	r := repository.NewRepository(s.pool)
	var linkUser *uuid.UUID
	intent := model.IntentLogin
	if in.Intent == model.IntentLink {
		// Connecting an account is sensitive: a stolen session must not add the thief's login.
		if err := s.requireRecentSignIn(ctx, r, in.LinkUserID, in.SessionID); err != nil {
			return "", "", err
		}
		intent, linkUser = model.IntentLink, &in.LinkUserID
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
		AppCodeChallenge: challenge, ExpiresAt: s.now().Add(model.OauthFlowTTL),
	}); err != nil {
		return "", "", fmt.Errorf("create oauth flow: %w", err)
	}
	return p.AuthURL(state, oauth2.S256ChallengeFromVerifier(verifier), nonce, s.callbackURL(p.Name())), state, nil
}

// --- helpers ---

// signIn starts a session and records the sign-in (shared by web, mobile and the email step).
func (s *Service) signIn(ctx context.Context, r repository.Repository, userID uuid.UUID, provider string, meta model.RequestMeta) (model.Tokens, error) {
	t, err := s.startSession(ctx, r, userID, provider, meta, s.now())
	if err != nil {
		return model.Tokens{}, err
	}
	if err := r.TouchLastLogin(ctx, userID); err != nil {
		return model.Tokens{}, fmt.Errorf("touch last login: %w", err)
	}
	return t, s.record(ctx, r, &userID, model.EventLoginSucceeded, provider, meta, nil)
}

func (s *Service) createPendingFlow(ctx context.Context, r repository.Repository, status, provider, client, redirect, tz string,
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
func (s *Service) requireRecentSignIn(ctx context.Context, r repository.Repository, userID, sessionID uuid.UUID) error {
	at, err := r.GetSessionAuthenticatedAt(ctx, gen.GetSessionAuthenticatedAtParams{ID: sessionID, UserID: userID})
	if repository.IsNoRows(err) {
		return model.ErrReauthRequired
	}
	if err != nil {
		return fmt.Errorf("load session: %w", err)
	}
	if s.now().Sub(at) > s.cfg.ReauthMaxAge {
		return model.ErrReauthRequired
	}
	return nil
}

// checkActive refuses a disabled account, and one whose email was never proven
// (sessions need both; see IsSessionActive).
func checkActive(ctx context.Context, r repository.Repository, userID uuid.UUID) error {
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return fmt.Errorf("load account: %w", err)
	}
	if acc.Status != "active" {
		return model.ErrAccountDisabled
	}
	if acc.EmailVerifiedAt == nil {
		return model.ErrEmailNotVerified
	}
	return nil
}

// oauthFailure turns an error into a redirect. Our own failures are logged;
// the client only sees a code.
func (s *Service) oauthFailure(ctx context.Context, redirect string, err error) model.OAuthResult {
	var ae *apperr.Error
	switch {
	case errors.Is(err, errProviderRejected):
		logger.FromContext(ctx).Warn("oauth sign-in rejected", "err", err)
		return model.OAuthResult{RedirectTo: redirect, ErrorCode: model.ErrOAuthFailed.Code}
	case errors.As(err, &ae) && ae.Kind != apperr.KindInternal:
		return model.OAuthResult{RedirectTo: redirect, ErrorCode: ae.Code}
	default:
		logger.FromContext(ctx).Error("oauth sign-in failed", "err", err)
		return model.OAuthResult{RedirectTo: redirect, ErrorCode: oauthErrServer}
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
	if client == model.ClientMobile {
		return "", model.ErrInvalidRedirect
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
