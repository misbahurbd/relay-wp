package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
)

func (h *Handler) listTemplates(ctx context.Context, in *dto.ListTemplatesInput) (*dto.InboxTemplatesOutput, error) {
	list, err := h.svc.Templates(ctx, in.WorkspaceID, in.ChannelType)
	if err != nil {
		return nil, err
	}
	return dto.ToTemplatesOutput(list), nil
}
