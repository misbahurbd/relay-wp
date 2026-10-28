package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
)

// Accounts gives other features read-only access to safe account fields
// (never password_hash). features/profiles uses it for GET /v1/me.
type Accounts struct {
	pool *pgxpool.Pool
}

// NewAccounts returns an Accounts reader. It has no dependency on Service, so
// profiles can be built before auth.
func NewAccounts(pool *pgxpool.Pool) *Accounts { return &Accounts{pool: pool} }

// AccountInfo returns the email, whether it's verified, and whether a password is set.
func (a *Accounts) AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error) {
	row, err := newRepository(a.pool).GetAccount(ctx, userID)
	if err != nil {
		return "", false, false, fmt.Errorf("load account: %w", err)
	}
	return row.Email, row.EmailVerifiedAt != nil, row.HasPassword, nil
}

// SessionActive reports whether the session behind an access token is still
// usable: not revoked, not expired, and its user not disabled.
// platform/middleware.RequireAuth calls it on every authenticated request.
func (a *Accounts) SessionActive(ctx context.Context, userID, sessionID uuid.UUID) (bool, error) {
	active, err := newRepository(a.pool).IsSessionActive(ctx, gen.IsSessionActiveParams{ID: sessionID, UserID: userID})
	if err != nil {
		return false, fmt.Errorf("check session: %w", err)
	}
	return active, nil
}
