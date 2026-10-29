package auth

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

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
