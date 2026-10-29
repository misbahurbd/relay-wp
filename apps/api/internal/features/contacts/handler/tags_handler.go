package handler

import (
	"context"
	"net/http"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/dto"
)

func (h *Handler) listTags(ctx context.Context, in *dto.WorkspaceInput) (*dto.TagsOutput, error) {
	tags, err := h.svc.ListTags(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.TagsOutput{Body: make([]dto.TagDTO, len(tags))}
	for i, t := range tags {
		out.Body[i] = dto.ToTagDTO(t)
	}
	return out, nil
}

func (h *Handler) createTag(ctx context.Context, in *dto.CreateTagInput) (*dto.TagOutput, error) {
	t, created, err := h.svc.CreateTag(ctx, in.WorkspaceID, in.Body.Name)
	if err != nil {
		return nil, err
	}
	out := &dto.TagOutput{Status: http.StatusOK, Body: dto.ToTagDTO(t)}
	if created {
		out.Status = http.StatusCreated
	}
	return out, nil
}
