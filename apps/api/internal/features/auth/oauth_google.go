package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// GoogleConfig holds Google sign-in credentials. The endpoint fields default to
// Google's; tests point them at a fake server.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string

	AuthURL  string
	TokenURL string
	JWKSURL  string
	Issuer   string
}

type google struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
}

// newGoogle returns nil when Google isn't configured.
func newGoogle(cfg GoogleConfig, client *http.Client) (Provider, error) {
	ok, err := credentials(cfg.ClientID, cfg.ClientSecret, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET")
	if !ok {
		return nil, err
	}
	def := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	authURL := def(cfg.AuthURL, "https://accounts.google.com/o/oauth2/v2/auth")
	tokenURL := def(cfg.TokenURL, "https://oauth2.googleapis.com/token")
	jwksURL := def(cfg.JWKSURL, "https://www.googleapis.com/oauth2/v3/certs")
	issuer := def(cfg.Issuer, "https://accounts.google.com")

	// Keys are fetched lazily on first use and cached; startup makes no network call.
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), client), jwksURL)
	return &google{
		oauth: oauth2.Config{
			ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret,
			Endpoint: oauth2.Endpoint{AuthURL: authURL, TokenURL: tokenURL, AuthStyle: oauth2.AuthStyleInParams},
			Scopes:   []string{oidc.ScopeOpenID, "email", "profile"},
		},
		verifier: oidc.NewVerifier(issuer, keys, &oidc.Config{ClientID: cfg.ClientID}),
		client:   client,
	}, nil
}

func (g *google) Name() string { return providerGoogle }

func (g *google) AuthURL(state, codeChallenge, nonce, redirectURI string) string {
	c := g.oauth
	c.RedirectURL = redirectURI
	return c.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

func (g *google) Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (ProviderIdentity, error) {
	c := g.oauth
	c.RedirectURL = redirectURI
	ctx = oidc.ClientContext(ctx, g.client)

	tok, err := c.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return ProviderIdentity{}, fmt.Errorf("%w: token exchange: %s", errProviderRejected, re.ErrorCode)
		}
		return ProviderIdentity{}, fmt.Errorf("google token exchange: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return ProviderIdentity{}, fmt.Errorf("%w: no id_token", errProviderRejected)
	}
	// Checks signature, issuer, audience and expiry.
	idToken, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("%w: id_token: %w", errProviderRejected, err)
	}
	if idToken.Nonce != nonce {
		return ProviderIdentity{}, fmt.Errorf("%w: nonce mismatch", errProviderRejected)
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		GivenName     string `json:"given_name"`
		FamilyName    string `json:"family_name"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return ProviderIdentity{}, fmt.Errorf("google claims: %w", err)
	}
	first, last := claims.GivenName, claims.FamilyName
	if first == "" && claims.Name != "" {
		first, last, _ = strings.Cut(claims.Name, " ")
	}
	return ProviderIdentity{
		Provider: providerGoogle, Subject: idToken.Subject,
		Email: claims.Email, EmailVerified: claims.Email != "" && claims.EmailVerified,
		FirstName: first, LastName: last, AvatarURL: claims.Picture,
	}, nil
}
