package team

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Resolver works out a member's access with one query per request and keeps
// nothing between requests (ADR 0002). It has no dependency on Service, so the
// middleware can be built first.
type Resolver struct {
	pool *pgxpool.Pool
}

// NewResolver returns a Resolver.
func NewResolver(pool *pgxpool.Pool) *Resolver { return &Resolver{pool: pool} }

// ResolveAccess implements middleware.AccessResolver.
func (r *Resolver) ResolveAccess(ctx context.Context, workspaceID, userID uuid.UUID) (authctx.Access, bool, bool, error) {
	row, err := newRepository(r.pool).ResolveAccess(ctx, gen.ResolveAccessParams{WorkspaceID: workspaceID, UserID: userID})
	if isNoRows(err) {
		return authctx.Access{}, false, false, nil
	}
	if err != nil {
		return authctx.Access{}, false, false, fmt.Errorf("resolve access: %w", err)
	}
	return toAccess(workspaceID, userID, row), row.Status == StatusSuspended, true, nil
}

func toAccess(workspaceID, userID uuid.UUID, row gen.ResolveAccessRow) authctx.Access {
	// Bits are 0–62 (permissions.bit CHECK), so the int64 masks are never negative.
	bits := (authctx.Bits(row.RoleBits) | authctx.Bits(row.GrantBits)) &^ authctx.Bits(row.DenyBits) //nolint:gosec // see above
	if row.IsOwner {
		bits = authctx.AllBits
	}
	return authctx.Access{
		WorkspaceID: workspaceID, UserID: userID, RoleID: row.RoleID, Owner: row.IsOwner, Bits: bits,
		Scope: row.Scope, Teams: row.TeamIds, Numbers: row.NumberIds,
		RoleVersion: row.RoleVersion, AccessVersion: row.AccessVersion,
	}
}
