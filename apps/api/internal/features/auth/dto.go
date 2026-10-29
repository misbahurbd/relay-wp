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
		// AcceptTerms must be true: the person agreed to the terms, privacy policy and WhatsApp policy.
		AcceptTerms bool `json:"accept_terms"`
	}
}

type verifyCodeInput struct {
	RequestHeaders
	Body struct {
		Email    string `json:"email" format:"email" maxLength:"254"`
		Code     string `json:"code" pattern:"^[0-9]{6}$" doc:"The 6-digit code from the verification email"`
		Password string `json:"password,omitempty" maxLength:"128" doc:"Required when the account has a password: the code alone doesn't prove who chose it"`
		Client   string `json:"client,omitempty" enum:"web,mobile" default:"web" doc:"web gets httpOnly cookies; mobile gets tokens in the body"`
	}
}

type emailInput struct {
	RequestHeaders
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254"`
	}
}

type verifyEmailInput struct {
	RequestHeaders
	Body struct {
		Token    string `json:"token" minLength:"1" maxLength:"256"`
		Password string `json:"password,omitempty" maxLength:"128" doc:"Required when the account has a password: the link alone doesn't prove who chose it"`
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

// --- Social sign-in ---

type providersOutput struct {
	Body struct {
		Providers []string `json:"providers" doc:"Social sign-in providers this server offers" example:"[\"google\",\"facebook\"]"`
	}
}

type oauthStartInput struct {
	Provider      string `path:"provider" enum:"google,facebook,apple"`
	Client        string `query:"client" enum:"web,mobile" default:"web" doc:"web ends with auth cookies; mobile ends at redirect_to with a one-time code"`
	RedirectTo    string `query:"redirect_to" maxLength:"2048" doc:"Where to land afterwards. Any page of the web app, or an exact AUTH_ALLOWED_REDIRECTS entry (required for mobile)."`
	Timezone      string `query:"timezone" maxLength:"64" doc:"IANA timezone, used if this sign-in creates the account"`
	CodeChallenge string `query:"code_challenge" maxLength:"64" doc:"Mobile only: PKCE S256 challenge of the verifier the app sends to POST /v1/auth/oauth/token"`
}

type oauthLinkInput struct {
	Provider   string `path:"provider" enum:"google,facebook,apple"`
	RedirectTo string `query:"redirect_to" maxLength:"2048" doc:"Settings page to return to"`
}

type oauthCallbackInput struct {
	RequestHeaders
	Provider    string `path:"provider" enum:"google,facebook,apple"`
	Code        string `query:"code" maxLength:"2048"`
	State       string `query:"state" maxLength:"256"`
	Error       string `query:"error" maxLength:"256" doc:"Set by the provider when the person cancels or it fails"`
	StateCookie string `cookie:"relay_oauth" doc:"Binds the attempt to the browser that started it"`
}

type redirectOutput struct {
	Status       int
	Location     string        `header:"Location"`
	CacheControl string        `header:"Cache-Control"`
	SetCookie    []http.Cookie `header:"Set-Cookie"`
}

type oauthCompleteInput struct {
	RequestHeaders
	Body struct {
		Token        string `json:"token" minLength:"1" maxLength:"256" doc:"complete_token from the callback redirect"`
		Email        string `json:"email" format:"email" maxLength:"254"`
		CodeVerifier string `json:"code_verifier,omitempty" maxLength:"128" doc:"Mobile only: the PKCE verifier"`
	}
}

type oauthCompleteVerifyInput struct {
	RequestHeaders
	Body struct {
		Token        string `json:"token" minLength:"1" maxLength:"256" doc:"complete_token from the callback redirect"`
		Code         string `json:"code" pattern:"^[0-9]{6}$" doc:"The 6-digit code emailed by POST /v1/auth/oauth/complete"`
		CodeVerifier string `json:"code_verifier,omitempty" maxLength:"128" doc:"Mobile only: the PKCE verifier"`
	}
}

type oauthTokenInput struct {
	RequestHeaders
	Body struct {
		Code         string `json:"code" minLength:"1" maxLength:"256" doc:"code from the callback redirect"`
		CodeVerifier string `json:"code_verifier" minLength:"43" maxLength:"128" doc:"PKCE verifier matching the code_challenge sent to start"`
	}
}

type setPasswordInput struct {
	RequestHeaders
	Body struct {
		NewPassword string `json:"new_password" minLength:"1" maxLength:"128"`
	}
}

type identityDTO struct {
	Provider   string    `json:"provider" enum:"google,facebook,apple"`
	Email      string    `json:"email,omitempty" doc:"What the provider reported"`
	LastUsedAt time.Time `json:"last_used_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type identitiesOutput struct {
	Body struct {
		HasPassword bool          `json:"has_password"`
		Identities  []identityDTO `json:"identities"`
	}
}

type identityProviderInput struct {
	RequestHeaders
	Provider string `path:"provider" enum:"google,facebook,apple"`
}

type dataDeletionInput struct {
	RequestHeaders
	RawBody []byte `contentType:"application/x-www-form-urlencoded"`
}

type dataDeletionOutput struct {
	Body struct {
		URL              string `json:"url" doc:"Page where the person can check the deletion status"`
		ConfirmationCode string `json:"confirmation_code"`
	}
}

type inviteSignupInput struct {
	RequestHeaders
	Body struct {
		Token       string  `json:"token" minLength:"20" maxLength:"100" doc:"From the invite link's #t= fragment"`
		Password    string  `json:"password" minLength:"1" maxLength:"128"`
		FirstName   string  `json:"first_name" minLength:"1" maxLength:"100"`
		LastName    *string `json:"last_name,omitempty" maxLength:"100"`
		Timezone    string  `json:"timezone,omitempty" maxLength:"64"`
		AcceptTerms bool    `json:"accept_terms"`
	}
}

type inviteSignupOutput struct {
	SetCookie []http.Cookie `header:"Set-Cookie"`
	Body      struct {
		WorkspaceID *uuid.UUID `json:"workspace_id" nullable:"true" doc:"Null when the account was made but joining failed"`
		JoinError   *string    `json:"join_error" doc:"Why joining failed; the person is signed in anyway"`
	}
}
