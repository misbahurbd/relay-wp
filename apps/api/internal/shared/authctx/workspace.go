package authctx

import (
	"context"

	"github.com/google/uuid"
)

type workspaceKey struct{}

// WithWorkspace returns ctx carrying the member's access in the workspace the request names.
func WithWorkspace(ctx context.Context, a Access) context.Context {
	return context.WithValue(ctx, workspaceKey{}, a)
}

// WorkspaceID returns the workspace the request is scoped to, if any.
func WorkspaceID(ctx context.Context) (uuid.UUID, bool) {
	a, ok := ctx.Value(workspaceKey{}).(Access)
	return a.WorkspaceID, ok
}

// AccessFrom returns the signed-in member's access in the request's workspace, if any.
func AccessFrom(ctx context.Context) (Access, bool) {
	a, ok := ctx.Value(workspaceKey{}).(Access)
	return a, ok
}
