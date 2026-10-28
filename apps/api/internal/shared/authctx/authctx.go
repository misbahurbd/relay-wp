// Package authctx carries the signed-in user through a request's context.
package authctx

import (
	"context"

	"github.com/google/uuid"
)

// Cookie names used by the web app. Shared so middleware (reading) and the
// auth feature (setting) agree.
const (
	AccessCookie  = "relay_access"
	RefreshCookie = "relay_refresh"
)

type key struct{}

type identity struct {
	userID, sessionID uuid.UUID
}

// With returns ctx carrying the signed-in user and session.
func With(ctx context.Context, userID, sessionID uuid.UUID) context.Context {
	return context.WithValue(ctx, key{}, identity{userID: userID, sessionID: sessionID})
}

// UserID returns the signed-in user, if any.
func UserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(key{}).(identity)
	return id.userID, ok
}

// SessionID returns the current session, if any.
func SessionID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(key{}).(identity)
	return id.sessionID, ok
}
