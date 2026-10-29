// Package handler serves team's HTTP endpoints: it decodes requests, calls the service and encodes
// responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/service"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the team HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

var errBadCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

// actor is set by middleware.RequireWorkspace on every workspace route here.
func actor(ctx context.Context) authctx.Access {
	a, _ := authctx.AccessFrom(ctx)
	return a
}

func meta(ctx context.Context, h dto.MetaHeaders) model.Meta {
	return model.Meta{IP: middleware.ClientIP(ctx), UserAgent: h.UserAgent}
}
