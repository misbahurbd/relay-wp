// Package handler serves the inbox's HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
	now func() time.Time
}

// NewHandler builds the inbox HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc, now: func() time.Time { return time.Now().UTC() }}
}

// actor is set by middleware.RequireWorkspace on every route here.
func actor(ctx context.Context) authctx.Access {
	a, _ := authctx.AccessFrom(ctx)
	return a
}
