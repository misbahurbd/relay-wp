package workspaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the workspaces HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) list(ctx context.Context, _ *struct{}) (*listOutput, error) {
	list, err := h.svc.List(ctx, userID(ctx))
	if err != nil {
		return nil, err
	}
	out := &listOutput{}
	out.Body.Workspaces = make([]summaryDTO, 0, len(list))
	for _, s := range list {
		out.Body.Workspaces = append(out.Body.Workspaces, toSummaryDTO(s))
	}
	return out, nil
}

func (h *Handler) create(ctx context.Context, in *createInput) (*summaryOutput, error) {
	s, err := h.svc.Create(ctx, userID(ctx), CreateInput{Name: in.Body.Name, FirstGoal: in.Body.FirstGoal, StartMode: in.Body.StartMode})
	if err != nil {
		return nil, err
	}
	return &summaryOutput{Body: toSummaryDTO(s)}, nil
}

func (h *Handler) get(ctx context.Context, in *workspaceInput) (*workspaceOutput, error) {
	w, err := h.svc.Get(ctx, in.WorkspaceID, userID(ctx))
	if err != nil {
		return nil, err
	}
	return &workspaceOutput{Body: toWorkspaceDTO(w)}, nil
}

func (h *Handler) saveBusiness(ctx context.Context, in *businessInput) (*workspaceOutput, error) {
	w, err := h.svc.SaveBusiness(ctx, in.WorkspaceID, BusinessInput{
		Name: in.Body.Name, Industry: in.Body.Industry, Website: in.Body.Website,
		Country: in.Body.Country, TeamSize: in.Body.TeamSize, Goals: in.Body.Goals,
	})
	if err != nil {
		return nil, err
	}
	return &workspaceOutput{Body: toWorkspaceDTO(w)}, nil
}

// userID is set by middleware.RequireAuth on every route here.
func userID(ctx context.Context) uuid.UUID {
	id, _ := authctx.UserID(ctx)
	return id
}
