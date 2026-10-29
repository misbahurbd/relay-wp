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
)

// linkIdentity connects a provider to the signed-in user (intent=link from settings).
func (s *Service) linkIdentity(ctx context.Context, id ProviderIdentity, userID uuid.UUID, redirect string, meta model.RequestMeta) model.OAuthResult {
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		existing, err := r.GetIdentity(ctx, gen.GetIdentityParams{Provider: id.Provider, ProviderUserID: id.Subject})
		switch {
		case err == nil && existing.UserID != userID:
			return model.ErrIdentityInUse
		case err == nil:
			return nil // already connected to this user
		case !repository.IsNoRows(err):
			return fmt.Errorf("look up identity: %w", err)
		}
		return s.addIdentity(ctx, r, userID, id, meta)
	})
	if err != nil {
		return s.oauthFailure(ctx, redirect, err)
	}
	return model.OAuthResult{RedirectTo: redirect, Linked: id.Provider}
}

// --- Settings: identities and password ---

// ListIdentities returns the connected providers and whether a password is set.
func (s *Service) ListIdentities(ctx context.Context, userID uuid.UUID) (hasPassword bool, out []model.Identity, err error) {
	r := repository.NewRepository(s.pool)
	acc, err := r.GetAccount(ctx, userID)
	if err != nil {
		return false, nil, fmt.Errorf("load account: %w", err)
	}
	rows, err := r.ListIdentities(ctx, userID)
	if err != nil {
		return false, nil, fmt.Errorf("list identities: %w", err)
	}
	out = make([]model.Identity, 0, len(rows))
	for _, row := range rows {
		i := model.Identity{Provider: row.Provider, LastUsedAt: row.LastUsedAt, CreatedAt: row.CreatedAt}
		if row.Email != nil {
			i.Email = *row.Email
		}
		out = append(out, i)
	}
	return acc.HasPassword, out, nil
}

// UnlinkIdentity disconnects a provider. It needs a recent sign-in and refuses
// to remove the last way to sign in.
func (s *Service) UnlinkIdentity(ctx context.Context, userID, sessionID uuid.UUID, provider string, meta model.RequestMeta) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if err := s.requireRecentSignIn(ctx, r, userID, sessionID); err != nil {
			return err
		}
		n, err := r.DeleteIdentity(ctx, gen.DeleteIdentityParams{UserID: userID, Provider: provider})
		if err != nil {
			return fmt.Errorf("delete identity: %w", err)
		}
		if n == 0 {
			return model.ErrIdentityNotFound
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
			return model.ErrLastSignInMethod // rolls the delete back
		}
		return s.record(ctx, r, &userID, model.EventIdentityUnlinked, provider, meta, nil)
	})
}

// --- Meta data deletion ---

// FacebookDataDeletion handles Meta's data-deletion callback: it verifies the
// signed request, disconnects that Facebook identity and returns the status
// page URL and confirmation code Meta shows the person.
func (s *Service) FacebookDataDeletion(ctx context.Context, signedRequest string, meta model.RequestMeta) (statusURL, code string, err error) {
	fb, ok := s.oauth[model.ProviderFacebook].(*facebook)
	if !ok {
		return "", "", model.ErrProviderNotAvailable
	}
	fbUserID, err := parseSignedRequest(signedRequest, fb.secret)
	if err != nil {
		return "", "", ErrInvalidSignedRequest.Wrap(err)
	}
	code = uuid.NewString()
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		userID, err := r.DeleteIdentityByProviderUser(ctx, gen.DeleteIdentityByProviderUserParams{
			Provider: model.ProviderFacebook, ProviderUserID: fbUserID,
		})
		if repository.IsNoRows(err) {
			return nil // nothing stored for this person; still confirm
		}
		if err != nil {
			return fmt.Errorf("delete facebook identity: %w", err)
		}
		return s.record(ctx, r, &userID, model.EventIdentityUnlinked, model.ProviderFacebook, meta,
			map[string]any{"reason": "meta_data_deletion", "confirmation_code": code})
	})
	if err != nil {
		return "", "", err
	}
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + "/data-deletion?code=" + code, code, nil
}
