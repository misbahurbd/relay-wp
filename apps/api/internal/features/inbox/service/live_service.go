package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/realtime"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Subscribe returns the live-update signals the member may see (AD-8): the
// chats in their scope and channels, and mentions of them. The access is the
// one resolved for this request; the stream is refreshed every 15 minutes, so
// a change of access takes effect on reconnect.
func (s *Service) Subscribe(a authctx.Access) (<-chan realtime.Event, func(), error) {
	if s.live == nil {
		return nil, nil, model.ErrLiveUnavailable
	}
	ch, stop := s.live.Subscribe(a.WorkspaceID, func(ev realtime.Event) bool { return canSeeEvent(a, ev) })
	return ch, stop, nil
}

// notifyUser sends a signal addressed to one member.
func notifyUser(ctx context.Context, tx pgx.Tx, ws uuid.UUID, kind string, conversationID uuid.UUID, channelType string,
	channelID, userID uuid.UUID,
) error {
	return realtime.Notify(ctx, tx, realtime.InboxChannel, realtime.Event{
		WorkspaceID: ws, Kind: kind, ConversationID: &conversationID, ChannelType: channelType, ChannelID: &channelID,
		UserID: &userID,
	})
}
