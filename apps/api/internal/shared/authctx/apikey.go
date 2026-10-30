package authctx

import (
	"context"

	"github.com/google/uuid"
)

// APIKeyPrefix starts every API key, so middleware can tell a key from a JWT
// without a lookup (ADR 0011).
const APIKeyPrefix = "relay_live_"

type apiKeyKey struct{}

// WithAPIKey marks ctx as authenticated by an API key made in workspaceID.
func WithAPIKey(ctx context.Context, workspaceID uuid.UUID) context.Context {
	return context.WithValue(ctx, apiKeyKey{}, workspaceID)
}

// APIKeyWorkspace returns the workspace of the API key behind the request, if
// it was made with one rather than a session.
func APIKeyWorkspace(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(apiKeyKey{}).(uuid.UUID)
	return id, ok
}
