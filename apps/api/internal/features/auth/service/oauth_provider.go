package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
)

// Provider is one social sign-in provider. The registry (Providers) is an
// approved factory: see docs/adr/0001-oauth-provider-registry.md.
type Provider interface {
	Name() string
	// AuthURL is where the browser is sent to sign in. codeChallenge is the
	// PKCE S256 challenge; providers that don't support it ignore it, as they do nonce.
	AuthURL(state, codeChallenge, nonce, redirectURI string) string
	// Exchange trades the callback code for the person's identity.
	Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (ProviderIdentity, error)
}

// ProviderIdentity is what a provider tells us about the person, normalised.
type ProviderIdentity struct {
	Provider string
	Subject  string // Google/Apple "sub", Facebook user ID
	Email    string // may be empty (Facebook phone-number accounts)
	// EmailVerified is true only when the provider vouches for the address.
	EmailVerified bool
	FirstName     string
	LastName      string
	AvatarURL     string
}

// errProviderRejected is a failed exchange the user can fix by trying again
// (expired code, wrong nonce, revoked consent). Other errors are ours.
var errProviderRejected = errors.New("provider rejected the sign-in")

// Providers maps a provider name to its implementation. Only configured providers are present.
type Providers map[string]Provider

// ProvidersConfig holds credentials for every provider. A provider with no
// credentials is left out; one with half its credentials is a startup error.
type ProvidersConfig struct {
	Google   GoogleConfig
	Facebook FacebookConfig
}

// providerHTTPTimeout bounds every call to a provider.
const providerHTTPTimeout = 10 * time.Second

// NewProviders builds every configured provider once, at startup.
func NewProviders(cfg ProvidersConfig) (Providers, error) {
	client := &http.Client{Timeout: providerHTTPTimeout}
	out := Providers{}
	for _, name := range []string{model.ProviderGoogle, model.ProviderFacebook} {
		var (
			p   Provider
			err error
		)
		switch name {
		case model.ProviderGoogle:
			p, err = newGoogle(cfg.Google, client)
		case model.ProviderFacebook:
			p, err = newFacebook(cfg.Facebook, client)
		}
		if err != nil {
			return nil, fmt.Errorf("%s sign-in: %w", name, err)
		}
		if p != nil {
			out[name] = p
		}
	}
	return out, nil
}

// Names lists the configured providers.
func (p Providers) Names() []string {
	out := make([]string, 0, len(p))
	for _, name := range []string{model.ProviderGoogle, model.ProviderFacebook} {
		if _, ok := p[name]; ok {
			out = append(out, name)
		}
	}
	return out
}

// credentials reports whether both halves are set, or an error if only one is.
func credentials(id, secret, idVar, secretVar string) (bool, error) {
	switch {
	case id == "" && secret == "":
		return false, nil
	case id == "" || secret == "":
		return false, fmt.Errorf("set both %s and %s, or neither", idVar, secretVar)
	}
	return true, nil
}
