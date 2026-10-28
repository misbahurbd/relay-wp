package authctx

import (
	"context"

	"github.com/google/uuid"
)

type workspaceKey struct{}

type membership struct {
	workspaceID uuid.UUID
	role        string
}

// WithWorkspace returns ctx carrying the workspace in the URL and the user's role in it.
func WithWorkspace(ctx context.Context, workspaceID uuid.UUID, role string) context.Context {
	return context.WithValue(ctx, workspaceKey{}, membership{workspaceID: workspaceID, role: role})
}

// WorkspaceID returns the workspace the request is scoped to, if any.
func WorkspaceID(ctx context.Context) (uuid.UUID, bool) {
	m, ok := ctx.Value(workspaceKey{}).(membership)
	return m.workspaceID, ok
}

// Role returns the signed-in user's role in the request's workspace, if any.
func Role(ctx context.Context) (string, bool) {
	m, ok := ctx.Value(workspaceKey{}).(membership)
	return m.role, ok
}
