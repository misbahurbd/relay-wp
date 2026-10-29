package model

import (
	"net/url"
	"time"

	"github.com/google/uuid"
)

// OAuthStart begins a social sign-in or a link from settings.
type OAuthStart struct {
	Provider   string
	Intent     string // login or link
	LinkUserID uuid.UUID
	SessionID  uuid.UUID // link only: the session must have signed in recently
	Client     string
	RedirectTo string
	Timezone   string
	// AppCodeChallenge is the mobile app's PKCE S256 challenge (required for mobile).
	AppCodeChallenge string
}

// OAuthResult is where a callback sends the browser next. Exactly one outcome is set.
type OAuthResult struct {
	RedirectTo string
	Tokens     *Tokens // web sign-in: set cookies
	AppCode    string  // mobile sign-in: one-time code for POST /auth/oauth/token
	// CompleteToken means the email needs proving: ask for one (POST /auth/oauth/complete).
	CompleteToken string
	PendingEmail  string // the provider's email, to prefill that form
	Linked        string // provider connected from settings
	ErrorCode     string // sign-in failed; shown by the page at RedirectTo
}

// Identity is one connected social account, as shown to its owner.
type Identity struct {
	Provider   string
	Email      string
	LastUsedAt time.Time
	CreatedAt  time.Time
}

// URL is RedirectTo with the outcome added as query parameters.
func (r OAuthResult) URL() string {
	u, err := url.Parse(r.RedirectTo)
	if err != nil {
		return r.RedirectTo
	}
	q := u.Query()
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("error", r.ErrorCode)
	set("linked", r.Linked)
	set("code", r.AppCode)
	set("complete_token", r.CompleteToken)
	set("email", r.PendingEmail)
	u.RawQuery = q.Encode()
	return u.String()
}
