package model

import (
	"time"

	"github.com/google/uuid"
)

// Mention is a note that @mentioned the member (AD-12).
type Mention struct {
	MessageID      uuid.UUID
	ConversationID uuid.UUID
	Contact        ContactRef
	Body           string
	Author         *Person
	SeenAt         *time.Time
	CreatedAt      time.Time
}
