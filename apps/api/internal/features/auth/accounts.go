package auth

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
