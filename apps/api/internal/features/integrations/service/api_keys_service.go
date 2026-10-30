package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

var base64URL = base64.RawURLEncoding

// lastUsedEvery limits last_used_at writes to one a minute per key.
const lastUsedEvery = time.Minute

// keyPrefixShown is how much of a key the list shows before the dots.
const keyPrefixShown = len(authctx.APIKeyPrefix) + 4

func hashKey(key string) []byte {
	h := sha256.Sum256([]byte(key))
	return h[:]
}

// APIKeys lists the workspace's active keys, newest first.
func (s *Service) APIKeys(ctx context.Context, workspaceID uuid.UUID) ([]model.APIKey, error) {
	rows, err := repository.NewRepository(s.pool).ListAPIKeys(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	out := make([]model.APIKey, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.APIKey{
			ID: r.ID, Name: r.Name, App: r.App, Prefix: r.Prefix, Last4: r.Last4, CreatedBy: r.CreatedBy,
			CreatedByName: r.CreatedByName, LastUsedAt: r.LastUsedAt, CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

// CreateAPIKey makes a key that acts as userID in the workspace. The full key is returned only here.
func (s *Service) CreateAPIKey(ctx context.Context, workspaceID, userID uuid.UUID, name string, app *string) (model.NewAPIKey, error) {
	name = strings.TrimSpace(name)
	if app != nil && !slices.Contains(model.KeyApps, *app) {
		return model.NewAPIKey{}, model.ErrUnknownProvider
	}
	var out model.NewAPIKey
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		// One workspace's key creations take turns, so the limit holds under concurrency.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1::text, 0))`, "api_keys:"+workspaceID.String()); err != nil {
			return fmt.Errorf("lock api keys: %w", err)
		}
		n, err := repo.CountActiveAPIKeys(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("count api keys: %w", err)
		}
		if n >= model.MaxAPIKeys {
			return model.ErrTooManyKeys
		}
		out, err = s.insertKey(ctx, repo, workspaceID, userID, name, app)
		return err
	})
	return out, err
}

// RollAPIKey replaces a key: a new one with the same name and app, and the old one revoked at once.
func (s *Service) RollAPIKey(ctx context.Context, workspaceID, userID, keyID uuid.UUID) (model.NewAPIKey, error) {
	var out model.NewAPIKey
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		old, err := repo.GetActiveAPIKey(ctx, gen.GetActiveAPIKeyParams{WorkspaceID: workspaceID, ID: keyID})
		if repository.IsNoRows(err) {
			return model.ErrKeyNotFound
		}
		if err != nil {
			return fmt.Errorf("get api key: %w", err)
		}
		if _, err := repo.RevokeAPIKey(ctx, gen.RevokeAPIKeyParams{WorkspaceID: workspaceID, ID: keyID}); err != nil {
			return fmt.Errorf("revoke api key: %w", err)
		}
		out, err = s.insertKey(ctx, repo, workspaceID, userID, old.Name, old.App)
		return err
	})
	return out, err
}

func (s *Service) insertKey(ctx context.Context, repo repository.Repository, workspaceID, userID uuid.UUID, name string, app *string) (model.NewAPIKey, error) {
	secret, err := randomToken(32)
	if err != nil {
		return model.NewAPIKey{}, err
	}
	key := authctx.APIKeyPrefix + secret
	id, err := uuid.NewV7()
	if err != nil {
		return model.NewAPIKey{}, fmt.Errorf("new id: %w", err)
	}
	k := model.APIKey{
		ID: id, Name: name, App: app, Prefix: key[:keyPrefixShown], Last4: key[len(key)-4:],
		CreatedBy: userID, CreatedAt: s.now(),
	}
	err = repo.CreateAPIKey(ctx, gen.CreateAPIKeyParams{
		ID: id, WorkspaceID: workspaceID, Name: name, App: app, Prefix: k.Prefix, Last4: k.Last4,
		TokenHash: hashKey(key), CreatedBy: userID,
	})
	if err != nil {
		return model.NewAPIKey{}, fmt.Errorf("create api key: %w", err)
	}
	return model.NewAPIKey{APIKey: k, Key: key}, nil
}

// RevokeAPIKey stops a key working on the next request.
func (s *Service) RevokeAPIKey(ctx context.Context, workspaceID, keyID uuid.UUID) error {
	n, err := repository.NewRepository(s.pool).RevokeAPIKey(ctx, gen.RevokeAPIKeyParams{WorkspaceID: workspaceID, ID: keyID})
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	if n == 0 {
		return model.ErrKeyNotFound
	}
	return nil
}

// VerifyAPIKey resolves a key to its workspace and creator for middleware.RequireAuth.
// The creator's current access is checked next, by RequireWorkspace.
func (s *Service) VerifyAPIKey(ctx context.Context, key string) (workspaceID, userID uuid.UUID, ok bool, err error) {
	if len(key) > 128 {
		return uuid.Nil, uuid.Nil, false, nil
	}
	repo := repository.NewRepository(s.pool)
	row, err := repo.GetAPIKeyByHash(ctx, hashKey(key))
	if repository.IsNoRows(err) {
		return uuid.Nil, uuid.Nil, false, nil
	}
	if err != nil {
		return uuid.Nil, uuid.Nil, false, fmt.Errorf("get api key: %w", err)
	}
	if row.LastUsedAt == nil || s.now().Sub(*row.LastUsedAt) > lastUsedEvery {
		if err := repo.TouchAPIKey(ctx, gen.TouchAPIKeyParams{WorkspaceID: row.WorkspaceID, ID: row.ID}); err != nil && s.log != nil {
			s.log.WarnContext(ctx, "touch api key", "err", err)
		}
	}
	return row.WorkspaceID, row.CreatedBy, true, nil
}
