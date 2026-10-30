package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
)

func (h *Handler) listConversations(ctx context.Context, in *dto.ListConversationsInput) (*dto.InboxConversationsOutput, error) {
	f, ok := in.ToModel()
	if !ok {
		return nil, model.ErrInvalidCursor
	}
	page, err := h.svc.List(ctx, actor(ctx), f)
	if err != nil {
		return nil, err
	}
	return dto.ToConversationsOutput(page), nil
}

func (h *Handler) conversation(c model.ConversationDetail, err error) (*dto.InboxConversationOutput, error) {
	if err != nil {
		return nil, err
	}
	return &dto.InboxConversationOutput{Body: dto.ToConversationDetailDTO(c, h.now())}, nil
}

func (h *Handler) getConversation(ctx context.Context, in *dto.ConversationPath) (*dto.InboxConversationOutput, error) {
	return h.conversation(h.svc.Get(ctx, actor(ctx), in.ConversationID))
}

func (h *Handler) assign(ctx context.Context, in *dto.AssignConversationInput) (*dto.InboxConversationOutput, error) {
	return h.conversation(h.svc.Assign(ctx, actor(ctx), in.ConversationID, in.Body.UserID))
}

func (h *Handler) setStatus(ctx context.Context, in *dto.SetConversationStatusInput) (*dto.InboxConversationOutput, error) {
	return h.conversation(h.svc.SetStatus(ctx, actor(ctx), in.ConversationID, in.Body.Status, in.Body.SnoozeUntil))
}

func (h *Handler) markRead(ctx context.Context, in *dto.ConversationPath) (*struct{}, error) {
	return nil, h.svc.MarkRead(ctx, actor(ctx), in.ConversationID)
}
