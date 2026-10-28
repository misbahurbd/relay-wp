package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
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

type signupInput struct {
	RequestHeaders
	Body struct {
		Email     string  `json:"email" format:"email" maxLength:"254"`
		Password  string  `json:"password" minLength:"1" maxLength:"128"`
		FirstName string  `json:"first_name" minLength:"1" maxLength:"100"`
		LastName  *string `json:"last_name,omitempty" maxLength:"100"`
		Timezone  string  `json:"timezone,omitempty" maxLength:"64" doc:"IANA timezone from the browser, e.g. Asia/Dhaka. Defaults to UTC."`
	}
}

type emailInput struct {
	RequestHeaders
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254"`
	}
}

type tokenInput struct {
	RequestHeaders
	Body struct {
		Token string `json:"token" minLength:"1" maxLength:"256"`
	}
}

type loginInput struct {
	RequestHeaders
	Body struct {
		Email    string `json:"email" maxLength:"254"`
		Password string `json:"password" maxLength:"128"`
		Client   string `json:"client,omitempty" enum:"web,mobile" default:"web" doc:"web gets httpOnly cookies; mobile gets tokens in the body"`
	}
}

type refreshInput struct {
	RequestHeaders
	Cookie  string `cookie:"relay_refresh" doc:"Web: refresh token cookie"`
	Origin  string `header:"Origin"`
	Referer string `header:"Referer"`
	Body    *struct {
		RefreshToken string `json:"refresh_token" maxLength:"256" doc:"Mobile: refresh token from the last sign-in or refresh"`
	}
}

type resetPasswordInput struct {
	RequestHeaders
	Body struct {
		Token       string `json:"token" minLength:"1" maxLength:"256"`
		NewPassword string `json:"new_password" minLength:"1" maxLength:"128"`
	}
}

type changePasswordInput struct {
	RequestHeaders
	Body struct {
		CurrentPassword string `json:"current_password" minLength:"1" maxLength:"128"`
		NewPassword     string `json:"new_password" minLength:"1" maxLength:"128"`
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
