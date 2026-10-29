package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

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
