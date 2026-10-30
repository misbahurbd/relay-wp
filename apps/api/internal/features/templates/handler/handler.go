// Package handler serves templates' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/templates/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the templates HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

var errBadCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

func (h *Handler) list(ctx context.Context, in *dto.ListInput) (*dto.ListOutput, error) {
	f, ok := in.Filter()
	if !ok {
		return nil, errBadCursor
	}
	items, next, counts, err := h.svc.List(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.ListOutput{}
	out.Body.Items = make([]dto.TemplateDTO, len(items))
	for i, t := range items {
		out.Body.Items[i] = dto.ToTemplateDTO(t)
	}
	if next != nil {
		c := dto.EncodeCursor(*next)
		out.Body.NextCursor = &c
	}
	out.Body.Counts = dto.CountsDTO(counts)
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *dto.TemplatePath) (*dto.TemplateOutput, error) {
	t, err := h.svc.Get(ctx, in.WorkspaceID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	return &dto.TemplateOutput{Body: dto.ToTemplateDTO(t)}, nil
}

func (h *Handler) create(ctx context.Context, in *dto.CreateInput) (*dto.TemplateOutput, error) {
	userID, _ := authctx.UserID(ctx)
	t, err := h.svc.Create(ctx, in.WorkspaceID, userID, in.Draft())
	if err != nil {
		return nil, err
	}
	return &dto.TemplateOutput{Body: dto.ToTemplateDTO(t)}, nil
}

func (h *Handler) update(ctx context.Context, in *dto.UpdateInput) (*dto.TemplateOutput, error) {
	t, err := h.svc.Update(ctx, in.WorkspaceID, in.TemplateID, in.Patch())
	if err != nil {
		return nil, err
	}
	return &dto.TemplateOutput{Body: dto.ToTemplateDTO(t)}, nil
}

func (h *Handler) uploadSample(ctx context.Context, in *dto.SampleInput) (*dto.TemplateOutput, error) {
	f := in.RawBody.Data().File
	defer func() { _ = f.Close() }()
	t, err := h.svc.UploadSample(ctx, in.WorkspaceID, in.TemplateID, f.ContentType, f, f.Size)
	if err != nil {
		return nil, err
	}
	return &dto.TemplateOutput{Body: dto.ToTemplateDTO(t)}, nil
}

func (h *Handler) submit(ctx context.Context, in *dto.TemplatePath) (*dto.TemplateOutput, error) {
	t, err := h.svc.Submit(ctx, in.WorkspaceID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	return &dto.TemplateOutput{Body: dto.ToTemplateDTO(t)}, nil
}

func (h *Handler) delete(ctx context.Context, in *dto.TemplatePath) (*struct{}, error) {
	return nil, h.svc.Delete(ctx, in.WorkspaceID, in.TemplateID)
}

func (h *Handler) sync(ctx context.Context, in *dto.WorkspaceInput) (*dto.SyncOutput, error) {
	n, err := h.svc.Sync(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.SyncOutput{}
	out.Body.Synced = n
	return out, nil
}
