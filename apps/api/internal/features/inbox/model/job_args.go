package model

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

// SendAttempts is how often a send is tried before the message is marked failed (AD-5).
const SendAttempts = 5

// SendArgs sends one pending outbound message.
type SendArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	MessageID   uuid.UUID `json:"message_id"`
}

// Kind is the job's stable name in the river_job table.
func (SendArgs) Kind() string { return "inbox_send" }

// InsertOpts sends each message once and retries transport errors.
func (SendArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: SendAttempts, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// ReactArgs sends the agent's reaction on a message; an empty emoji removes it.
type ReactArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	MessageID   uuid.UUID `json:"message_id"`
	Emoji       string    `json:"emoji"`
}

// Kind is the job's stable name in the river_job table.
func (ReactArgs) Kind() string { return "inbox_send_reaction" }

// InsertOpts retries a few times; a reaction that can't be sent is dropped.
func (ReactArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 3} }

// FetchMediaArgs downloads an inbound message's file from its channel into storage (AD-11).
type FetchMediaArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	MessageID   uuid.UUID `json:"message_id"`
}

// Kind is the job's stable name in the river_job table.
func (FetchMediaArgs) Kind() string { return "inbox_fetch_media" }

// InsertOpts fetches each file once, retrying while the channel or storage is down.
func (FetchMediaArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 8, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

// MarkReadArgs sends a read receipt (blue ticks on WhatsApp) for the customer's
// last message, on channels with read receipts.
type MarkReadArgs struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	ChannelType string    `json:"channel_type"`
	ChannelID   uuid.UUID `json:"channel_id"`
	ExternalID  string    `json:"external_id"`
}

// Kind is the job's stable name in the river_job table.
func (MarkReadArgs) Kind() string { return "inbox_mark_read" }

// InsertOpts tries once: a lost read receipt is not worth a retry (AD-5).
func (MarkReadArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 1} }

// UnsnoozeArgs reopens chats whose snooze has ended.
type UnsnoozeArgs struct{}

// Kind is the job's stable name in the river_job table.
func (UnsnoozeArgs) Kind() string { return "inbox_unsnooze" }
