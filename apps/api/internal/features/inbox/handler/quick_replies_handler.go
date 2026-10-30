package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

func (h *Handler) listQuickReplies(ctx context.Context, in *dto.InboxWorkspaceInput) (*dto.InboxQuickRepliesOutput, error) {
	list, err := h.svc.QuickReplies(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToQuickRepliesOutput(list), nil
}

func quickReply(q model.QuickReply, err error) (*dto.InboxQuickReplyOutput, error) {
	if err != nil {
		return nil, err
	}
	return dto.ToQuickReplyOutput(q), nil
}

func (h *Handler) createQuickReply(ctx context.Context, in *dto.CreateQuickReplyInput) (*dto.InboxQuickReplyOutput, error) {
	return quickReply(h.svc.CreateQuickReply(ctx, actor(ctx), in.ToModel()))
}

func (h *Handler) updateQuickReply(ctx context.Context, in *dto.UpdateQuickReplyInput) (*dto.InboxQuickReplyOutput, error) {
	return quickReply(h.svc.UpdateQuickReply(ctx, in.WorkspaceID, in.QuickReplyID, in.ToModel()))
}

func (h *Handler) deleteQuickReply(ctx context.Context, in *dto.QuickReplyPath) (*struct{}, error) {
	return nil, h.svc.DeleteQuickReply(ctx, in.WorkspaceID, in.QuickReplyID)
}
