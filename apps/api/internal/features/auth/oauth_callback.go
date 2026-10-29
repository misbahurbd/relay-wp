package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

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
