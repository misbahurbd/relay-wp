package model

import (
	"time"

	"github.com/google/uuid"
)

// Quick reply limits (section 5).
const (
	MaxShortcutLen        = 32
	MaxQuickReplyTitleLen = 80
	MaxQuickReplyBodyLen  = 1024
)

// QuickReply is a canned reply for the composer.
type QuickReply struct {
	ID        uuid.UUID
	Shortcut  string
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// QuickReplyInput creates a quick reply (every field) or changes one (nil keeps).
type QuickReplyInput struct {
	Shortcut *string
	Title    *string
	Body     *string
}
