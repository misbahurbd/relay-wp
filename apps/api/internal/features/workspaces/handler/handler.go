// Package handler serves workspaces' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/workspaces/service"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the workspaces HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) list(ctx context.Context, _ *struct{}) (*dto.ListOutput, error) {
	list, err := h.svc.List(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	out := &dto.ListOutput{}
	out.Body.Workspaces = make([]dto.SummaryDTO, 0, len(list))
	for _, s := range list {
		out.Body.Workspaces = append(out.Body.Workspaces, dto.ToSummaryDTO(s))
	}
	return out, nil
}

func (h *Handler) create(ctx context.Context, in *dto.CreateInput) (*dto.SummaryOutput, error) {
	s, err := h.svc.Create(ctx, userID(ctx), model.CreateInput{Name: in.Body.Name, FirstGoal: in.Body.FirstGoal, StartMode: in.Body.StartMode})
	if err != nil {
		return nil, err
	}
	return &dto.SummaryOutput{Body: dto.ToSummaryDTO(s)}, nil
}

func (h *Handler) get(ctx context.Context, in *dto.WorkspaceInput) (*dto.WorkspaceOutput, error) {
	w, err := h.svc.Get(ctx, in.WorkspaceID, userID(ctx))
	if err != nil {
		return nil, err
	}
	return &dto.WorkspaceOutput{Body: dto.ToWorkspaceDTO(w)}, nil
}

func (h *Handler) saveBusiness(ctx context.Context, in *dto.BusinessInput) (*dto.WorkspaceOutput, error) {
	w, err := h.svc.SaveBusiness(ctx, in.WorkspaceID, model.BusinessInput{
		Name: in.Body.Name, Industry: in.Body.Industry, Website: in.Body.Website,
		Country: in.Body.Country, TeamSize: in.Body.TeamSize, Goals: in.Body.Goals,
	})
	if err != nil {
		return nil, err
	}
	return &dto.WorkspaceOutput{Body: dto.ToWorkspaceDTO(w)}, nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
