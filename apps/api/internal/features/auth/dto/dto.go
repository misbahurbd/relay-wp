// Package dto holds auth's HTTP request and response shapes and the converters from domain types.
package dto

import (
	"net/http"
	"time"
)

// RequestHeaders are read on every auth call for sessions and the event log.
type RequestHeaders struct {
	UserAgent  string `header:"User-Agent"`
	DeviceName string `header:"X-Device-Name" doc:"Device name sent by the mobile app, e.g. iPhone 15"`
}

type MessageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func Message202(msg string) *MessageOutput {
	out := &MessageOutput{}
	out.Body.Message = msg
	return out
}

type EmailInput struct {
	RequestHeaders
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254"`
	}
}

type TokenBody struct {
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	AccessToken      string    `json:"access_token,omitempty" doc:"Mobile only"`
	RefreshToken     string    `json:"refresh_token,omitempty" doc:"Mobile only"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type TokenOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      TokenBody
}

type CookieOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type AuthedInput struct {
	RequestHeaders
}
