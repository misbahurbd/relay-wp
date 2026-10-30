// Package realtime fans database notifications out to live connections (ADR 0009).
//
// Writers call Notify inside their transaction, so Postgres delivers the
// notification only when the write commits. Each API instance runs one Hub: it
// LISTENs on one dedicated connection from the direct (unpooled) pool and passes
// every event to the subscribers of its workspace whose filter accepts it.
// Events are signals, never content: clients refetch through the normal,
// scoped endpoints.
package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// InboxChannel is the channel the inbox notifies on.
const InboxChannel = "inbox_events"

// Event is one signal. The IDs say which resource changed and who may see it,
// so a subscriber's filter can apply the member's scope without a query.
type Event struct {
	WorkspaceID    uuid.UUID  `json:"ws"`
	Kind           string     `json:"kind"`
	ConversationID *uuid.UUID `json:"conversation_id,omitempty"`
	TeamID         *uuid.UUID `json:"team_id,omitempty"`
	AssigneeID     *uuid.UUID `json:"assignee_id,omitempty"`
	// ChannelType and ChannelID are the conversation's channel (ADR 0010).
	ChannelType string     `json:"channel_type,omitempty"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty"`
	// UserID addresses the event to one member, for example a mention.
	UserID *uuid.UUID `json:"user_id,omitempty"`
}

// Execer runs a statement; a pgx.Tx satisfies it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// maxPayload is Postgres' NOTIFY payload limit, less a little room.
const maxPayload = 7900

// Notify sends ev on channel through tx. It is delivered when tx commits and
// dropped if tx rolls back.
func Notify(ctx context.Context, tx Execer, channel string, ev Event) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("realtime: encode event: %w", err)
	}
	if len(payload) > maxPayload {
		return fmt.Errorf("realtime: payload of %d bytes is too large", len(payload))
	}
	if _, err := tx.Exec(ctx, "SELECT pg_notify($1, $2)", channel, string(payload)); err != nil {
		return fmt.Errorf("realtime: notify: %w", err)
	}
	return nil
}
