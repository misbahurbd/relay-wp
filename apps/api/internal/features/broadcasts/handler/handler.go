// Package handler serves broadcasts' HTTP endpoints: it decodes requests, calls the service and
// encodes responses. Routes are registered in routes.go.
package handler

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the broadcasts HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

var errBadCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

func (h *Handler) list(ctx context.Context, in *dto.ListInput) (*dto.ListOutput, error) {
	f, ok := in.Filter()
	if !ok {
		return nil, errBadCursor
	}
	items, next, err := h.svc.List(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.ListOutput{}
	out.Body.Items = make([]dto.BroadcastSummaryDTO, len(items))
	for i, b := range items {
		out.Body.Items[i] = dto.ToSummaryDTO(b)
	}
	if next != nil {
		c := dto.EncodeCursor(*next)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *dto.BroadcastPath) (*dto.BroadcastOutput, error) {
	return output(h.svc.Get(ctx, in.WorkspaceID, in.BroadcastID))
}

func (h *Handler) create(ctx context.Context, in *dto.CreateInput) (*dto.BroadcastOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return output(h.svc.Create(ctx, in.WorkspaceID, userID, in.Body.Input()))
}

func (h *Handler) update(ctx context.Context, in *dto.UpdateInput) (*dto.BroadcastOutput, error) {
	return output(h.svc.Update(ctx, in.WorkspaceID, in.BroadcastID, in.Body.Input()))
}

func (h *Handler) delete(ctx context.Context, in *dto.BroadcastPath) (*struct{}, error) {
	return nil, h.svc.Delete(ctx, in.WorkspaceID, in.BroadcastID)
}

func (h *Handler) duplicate(ctx context.Context, in *dto.BroadcastPath) (*dto.BroadcastOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return output(h.svc.Duplicate(ctx, in.WorkspaceID, userID, in.BroadcastID))
}

func (h *Handler) send(ctx context.Context, in *dto.SendInput) (*dto.BroadcastOutput, error) {
	return output(h.svc.Send(ctx, in.WorkspaceID, in.BroadcastID, in.Body.ScheduledAt))
}

func (h *Handler) cancel(ctx context.Context, in *dto.BroadcastPath) (*dto.BroadcastOutput, error) {
	return output(h.svc.Cancel(ctx, in.WorkspaceID, in.BroadcastID))
}

func (h *Handler) previewAudience(ctx context.Context, in *dto.AudienceInput) (*dto.AudienceOutput, error) {
	c, err := h.svc.PreviewAudience(ctx, in.WorkspaceID, in.Body.Audience(), in.Body.NumberID)
	if err != nil {
		return nil, err
	}
	out := &dto.AudienceOutput{}
	out.Body.Included, out.Body.LeftOut, out.Body.Recent, out.Body.TierLimit = c.Included, c.LeftOut, c.Recent, c.TierLimit
	return out, nil
}

func (h *Handler) recipients(ctx context.Context, in *dto.RecipientsInput) (*dto.RecipientsOutput, error) {
	var after *uuid.UUID
	if in.Cursor != "" {
		id, ok := dto.DecodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		after = &id
	}
	var filter *string
	if in.Filter != "" {
		filter = &in.Filter
	}
	items, next, err := h.svc.Recipients(ctx, in.WorkspaceID, in.BroadcastID, filter, after, in.Limit)
	if err != nil {
		return nil, err
	}
	out := &dto.RecipientsOutput{}
	out.Body.Items = make([]dto.RecipientDTO, len(items))
	for i, r := range items {
		out.Body.Items[i] = dto.ToRecipientDTO(r)
	}
	if next != nil {
		c := dto.EncodeCursor(*next)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func output(b model.Broadcast, err error) (*dto.BroadcastOutput, error) {
	if err != nil {
		return nil, err
	}
	return &dto.BroadcastOutput{Body: dto.ToBroadcastDTO(b)}, nil
}
