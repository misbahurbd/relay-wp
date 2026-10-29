package team

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the team HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

var errBadCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

// actor is set by middleware.RequireWorkspace on every workspace route here.
func actor(ctx context.Context) authctx.Access {
	a, _ := authctx.AccessFrom(ctx)
	return a
}

func meta(ctx context.Context, h metaHeaders) Meta {
	return Meta{IP: middleware.ClientIP(ctx), UserAgent: h.UserAgent}
}
