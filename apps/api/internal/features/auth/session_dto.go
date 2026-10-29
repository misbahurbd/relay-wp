package auth

import (
	"time"

	"github.com/google/uuid"
)

type refreshInput struct {
	RequestHeaders
	Cookie  string `cookie:"relay_refresh" doc:"Web: refresh token cookie"`
	Origin  string `header:"Origin"`
	Referer string `header:"Referer"`
	Body    *struct {
		RefreshToken string `json:"refresh_token" maxLength:"256" doc:"Mobile: refresh token from the last sign-in or refresh"`
	}
}

type sessionIDInput struct {
	RequestHeaders
	ID uuid.UUID `path:"id"`
}

type sessionDTO struct {
	ID         uuid.UUID `json:"id"`
	DeviceName string    `json:"device_name" example:"Chrome on macOS"`
	DeviceType string    `json:"device_type" enum:"desktop,mobile,tablet,app"`
	Provider   string    `json:"provider" enum:"password,google,facebook,apple"`
	Client     string    `json:"client" enum:"web,mobile"`
	LastIP     string    `json:"last_ip,omitempty"`
	LastUsedAt time.Time `json:"last_used_at"`
	CreatedAt  time.Time `json:"created_at"`
	Current    bool      `json:"current" doc:"True for the device making this request"`
}

type sessionsOutput struct {
	Body struct {
		Sessions []sessionDTO `json:"sessions"`
	}
}

type eventDTO struct {
	ID        uuid.UUID `json:"id"`
	Event     string    `json:"event" example:"login_succeeded"`
	Provider  string    `json:"provider,omitempty"`
	IP        string    `json:"ip,omitempty"`
	Device    string    `json:"device" example:"Safari on iPhone"`
	CreatedAt time.Time `json:"created_at"`
}

type eventsOutput struct {
	Body struct {
		Events []eventDTO `json:"events"`
	}
}
