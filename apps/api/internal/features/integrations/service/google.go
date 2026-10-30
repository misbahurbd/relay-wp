package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Google's OAuth endpoints.
const (
	GoogleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	GoogleTokenURL    = "https://oauth2.googleapis.com/token" //nolint:gosec // a URL, not a credential
	GoogleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
	googleScopes      = "https://www.googleapis.com/auth/spreadsheets email"
)

// GoogleConfig is the Google app used for Sheets (the same one as Google sign-in).
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is {API_BASE_URL}/v1/integrations/google/callback.
	RedirectURL string
	// Endpoint overrides for tests; empty means Google's.
	AuthURL, TokenURL, UserInfoURL string
}

func (c GoogleConfig) endpoint(override, def string) string {
	if override != "" {
		return override
	}
	return def
}

// AuthorizeGoogle returns Google's consent URL and the nonce the handler puts in a cookie.
func (s *Service) AuthorizeGoogle(workspaceID, userID uuid.UUID) (authURL, nonce string, err error) {
	g := s.cfg.Google
	if g.ClientID == "" {
		return "", "", model.ErrGoogleUnavailable
	}
	state, nonce, err := s.newState(workspaceID, userID, model.ProviderGoogleSheets, "", true)
	if err != nil {
		return "", "", err
	}
	q := url.Values{
		"client_id": {g.ClientID}, "redirect_uri": {g.RedirectURL}, "response_type": {"code"},
		"scope": {googleScopes}, "access_type": {"offline"}, "prompt": {"consent"},
		"include_granted_scopes": {"true"}, "state": {state},
	}
	return g.endpoint(g.AuthURL, GoogleAuthURL) + "?" + q.Encode(), nonce, nil
}

// GoogleCallback finishes Google sign-in and returns where to send the person: the
// Integrations page, with ?connected=google_sheets or ?integration_error=<code>.
func (s *Service) GoogleCallback(ctx context.Context, code, state, nonce, providerError string) string {
	back := s.integrationsPage()
	fail := func(err error) string {
		return s.connectFailure(back, model.ProviderGoogleSheets, err, model.ErrGoogleExchange.Code)
	}
	st, err := s.readState(state, model.ProviderGoogleSheets, nonce)
	if err != nil {
		return fail(err)
	}
	if providerError != "" || code == "" {
		return fail(model.ErrGoogleExchange)
	}
	tok, err := s.exchangeGoogle(ctx, url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {s.cfg.Google.RedirectURL},
	})
	if err != nil {
		return fail(err)
	}
	if tok.RefreshToken == "" {
		return fail(model.ErrNoRefreshToken)
	}
	email, err := s.googleEmail(ctx, tok.AccessToken)
	if err != nil {
		return fail(err)
	}
	if err := s.saveConnection(ctx, st.WorkspaceID, st.UserID, model.ProviderGoogleSheets, email,
		storeConfig{Email: email}, storeCredentials{RefreshToken: tok.RefreshToken}); err != nil {
		return fail(err)
	}
	return back + "?connected=" + model.ProviderGoogleSheets
}

type googleToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func (s *Service) refreshGoogle(ctx context.Context, refreshToken string) (googleToken, error) {
	if s.cfg.Google.ClientID == "" {
		return googleToken{}, model.ErrGoogleUnavailable
	}
	return s.exchangeGoogle(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
}

func (s *Service) exchangeGoogle(ctx context.Context, form url.Values) (googleToken, error) {
	g := s.cfg.Google
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.endpoint(g.TokenURL, GoogleTokenURL), strings.NewReader(form.Encode()))
	if err != nil {
		return googleToken{}, fmt.Errorf("google token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.google.Do(req)
	if err != nil {
		return googleToken{}, fmt.Errorf("google token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return googleToken{}, model.ErrGoogleExchange
	}
	if resp.StatusCode != http.StatusOK {
		return googleToken{}, fmt.Errorf("google token: status %d", resp.StatusCode)
	}
	var tok googleToken
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&tok); err != nil {
		return googleToken{}, fmt.Errorf("google token: %w", err)
	}
	return tok, nil
}

func (s *Service) googleEmail(ctx context.Context, accessToken string) (string, error) {
	g := s.cfg.Google
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.endpoint(g.UserInfoURL, GoogleUserInfoURL), nil)
	if err != nil {
		return "", fmt.Errorf("google userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := s.google.Do(req)
	if err != nil {
		return "", fmt.Errorf("google userinfo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}
	var info struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&info); err != nil {
		return "", fmt.Errorf("google userinfo: %w", err)
	}
	return info.Email, nil
}

func asAppErr(err error) *apperr.Error {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return ae
	}
	return nil
}
