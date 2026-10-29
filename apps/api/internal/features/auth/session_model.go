package auth

import (
	"time"

	"github.com/google/uuid"
)

// Session is one signed-in device, as shown to its owner.
type Session struct {
	ID         uuid.UUID
	Provider   string
	Client     string
	DeviceName string
	DeviceType string
	LastIP     string
	LastUsedAt time.Time
	CreatedAt  time.Time
	Current    bool
}

// Event is one entry of a user's security activity.
type Event struct {
	ID        uuid.UUID
	Event     string
	Provider  string
	IP        string
	Device    string
	CreatedAt time.Time
}
