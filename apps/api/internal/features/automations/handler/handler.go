// Package handler serves automations' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the automations HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) list(ctx context.Context, in *dto.WorkspaceInput) (*dto.ListOutput, error) {
	items, err := h.svc.List(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.ListOutput{}
	out.Body.Items = make([]dto.AutomationDTO, len(items))
	for i, a := range items {
		out.Body.Items[i] = dto.ToAutomationDTO(a)
	}
	return out, nil
}

func (h *Handler) save(ctx context.Context, in *dto.SaveInput) (*dto.AutomationOutput, error) {
	kind := model.Kind(in.Kind)
	settings := in.Body.For(kind)
	if settings == nil {
		return nil, model.ErrInvalidSettings.WithDetails(apperr.Detail{Field: in.Kind, Message: "Send this automation's settings"})
	}
	userID, _ := authctx.UserID(ctx)
	a, err := h.svc.Save(ctx, in.WorkspaceID, userID, kind, settings)
	if err != nil {
		return nil, err
	}
	return &dto.AutomationOutput{Body: dto.ToAutomationDTO(a)}, nil
}

func (h *Handler) setEnabled(ctx context.Context, in *dto.SetEnabledInput) (*dto.AutomationOutput, error) {
	userID, _ := authctx.UserID(ctx)
	a, err := h.svc.SetEnabled(ctx, in.WorkspaceID, userID, model.Kind(in.Kind), in.Body.Enabled)
	if err != nil {
		return nil, err
	}
	return &dto.AutomationOutput{Body: dto.ToAutomationDTO(a)}, nil
}
