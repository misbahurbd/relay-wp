package auth

import (
	"net/http"
	"time"
)

// RequestHeaders are read on every auth call for sessions and the event log.
type RequestHeaders struct {
	UserAgent  string `header:"User-Agent"`
	DeviceName string `header:"X-Device-Name" doc:"Device name sent by the mobile app, e.g. iPhone 15"`
}

type messageOutput struct {
	Body struct {
		Message string `json:"message"`
	}
}

func message202(msg string) *messageOutput {
	out := &messageOutput{}
	out.Body.Message = msg
	return out
}

type emailInput struct {
	RequestHeaders
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254"`
	}
}

type tokenBody struct {
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	AccessToken      string    `json:"access_token,omitempty" doc:"Mobile only"`
	RefreshToken     string    `json:"refresh_token,omitempty" doc:"Mobile only"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type tokenOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      tokenBody
}

type cookieOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
}

type authedInput struct {
	RequestHeaders
}
