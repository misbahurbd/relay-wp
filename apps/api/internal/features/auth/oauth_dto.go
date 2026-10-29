package auth

import (
	"net/http"
	"time"
)

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
