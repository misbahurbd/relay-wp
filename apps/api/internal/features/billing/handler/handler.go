// Package handler serves billing's HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go; each area's handlers live in
// <area>_handler.go (track T6) and the payment webhook in webhook_handler.go (track T5).
package handler

import (
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the billing HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }
