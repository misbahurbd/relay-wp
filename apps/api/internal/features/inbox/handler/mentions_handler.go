package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
)

func (h *Handler) listMentions(ctx context.Context, _ *dto.InboxWorkspaceInput) (*dto.InboxMentionsOutput, error) {
	list, unseen, err := h.svc.Mentions(ctx, actor(ctx))
	if err != nil {
		return nil, err
	}
	return dto.ToMentionsOutput(list, unseen), nil
}

func (h *Handler) markMentionsSeen(ctx context.Context, in *dto.MarkMentionsSeenInput) (*dto.InboxMentionsSeenOutput, error) {
	n, err := h.svc.MarkMentionsSeen(ctx, actor(ctx), in.Body.MessageIDs)
	if err != nil {
		return nil, err
	}
	out := &dto.InboxMentionsSeenOutput{}
	out.Body.Updated = n
	return out, nil
}
