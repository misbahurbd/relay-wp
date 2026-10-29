package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"

	"golang.org/x/oauth2"
)

// facebookGraphVersion is the Graph API version every call uses. Meta supports
// each version for about two years; bump it before it is retired.
const facebookGraphVersion = "v24.0"

// FacebookConfig holds Facebook Login credentials. The endpoint fields default
// to Meta's; tests point them at a fake server.
type FacebookConfig struct {
	AppID     string
	AppSecret string

	AuthURL  string
	TokenURL string
	GraphURL string // base for /me, e.g. https://graph.facebook.com/v24.0
}

type facebook struct {
	oauth    oauth2.Config
	graphURL string
	secret   string
	client   *http.Client
}

// newFacebook returns nil when Facebook isn't configured.
func newFacebook(cfg FacebookConfig, client *http.Client) (Provider, error) {
	ok, err := credentials(cfg.AppID, cfg.AppSecret, "FACEBOOK_APP_ID", "FACEBOOK_APP_SECRET")
	if !ok {
		return nil, err
	}
	def := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	return &facebook{
		oauth: oauth2.Config{
			ClientID: cfg.AppID, ClientSecret: cfg.AppSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:   def(cfg.AuthURL, "https://www.facebook.com/"+facebookGraphVersion+"/dialog/oauth"),
				TokenURL:  def(cfg.TokenURL, "https://graph.facebook.com/"+facebookGraphVersion+"/oauth/access_token"),
				AuthStyle: oauth2.AuthStyleInParams,
			},
			Scopes: []string{"email", "public_profile"},
		},
		graphURL: strings.TrimRight(def(cfg.GraphURL, "https://graph.facebook.com/"+facebookGraphVersion), "/"),
		secret:   cfg.AppSecret,
		client:   client,
	}, nil
}

func (f *facebook) Name() string { return model.ProviderFacebook }

// AuthURL ignores codeChallenge and nonce: the flow is protected by state, the
// browser-binding cookie and the app secret on the server-side exchange.
func (f *facebook) AuthURL(state, _, _, redirectURI string) string {
	c := f.oauth
	c.RedirectURL = redirectURI
	return c.AuthCodeURL(state)
}

func (f *facebook) Exchange(ctx context.Context, code, _, _, redirectURI string) (ProviderIdentity, error) {
	c := f.oauth
	c.RedirectURL = redirectURI
	tok, err := c.Exchange(context.WithValue(ctx, oauth2.HTTPClient, f.client), code)
	if err != nil {
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			return ProviderIdentity{}, fmt.Errorf("%w: token exchange: %s", errProviderRejected, re.ErrorCode)
		}
		return ProviderIdentity{}, fmt.Errorf("facebook token exchange: %w", err)
	}

	// appsecret_proof proves the call comes from our server, not from someone
	// holding a leaked user token.
	mac := hmac.New(sha256.New, []byte(f.secret))
	mac.Write([]byte(tok.AccessToken))
	q := url.Values{
		"fields":          {"id,email,first_name,last_name,picture.width(256).height(256)"},
		"access_token":    {tok.AccessToken},
		"appsecret_proof": {hex.EncodeToString(mac.Sum(nil))},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.graphURL+"/me?"+q.Encode(), nil)
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("facebook profile request: %w", err)
	}
	res, err := f.client.Do(req)
	if err != nil {
		return ProviderIdentity{}, fmt.Errorf("facebook profile: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, res.Body)
		return ProviderIdentity{}, fmt.Errorf("%w: profile status %d", errProviderRejected, res.StatusCode)
	}

	var me struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Picture   struct {
			Data struct {
				URL          string `json:"url"`
				IsSilhouette bool   `json:"is_silhouette"`
			} `json:"data"`
		} `json:"picture"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&me); err != nil {
		return ProviderIdentity{}, fmt.Errorf("facebook profile: %w", err)
	}
	if me.ID == "" {
		return ProviderIdentity{}, fmt.Errorf("%w: profile has no id", errProviderRejected)
	}
	avatar := me.Picture.Data.URL
	if me.Picture.Data.IsSilhouette {
		avatar = "" // the default grey silhouette isn't a real photo
	}
	// Facebook never vouches for email: it is treated as unverified everywhere.
	return ProviderIdentity{
		Provider: model.ProviderFacebook, Subject: me.ID, Email: me.Email, EmailVerified: false,
		FirstName: me.FirstName, LastName: me.LastName, AvatarURL: avatar,
	}, nil
}

// parseSignedRequest verifies a Meta signed_request ("sig.payload", both
// base64url) with the app secret and returns the Facebook user ID in it.
func parseSignedRequest(signed, secret string) (string, error) {
	sigPart, payloadPart, ok := strings.Cut(signed, ".")
	if !ok {
		return "", errors.New("malformed signed_request")
	}
	sig, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(sigPart, "="))
	if err != nil {
		return "", errors.New("malformed signature")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadPart))
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return "", errors.New("bad signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(payloadPart, "="))
	if err != nil {
		return "", errors.New("malformed payload")
	}
	var body struct {
		Algorithm string `json:"algorithm"`
		UserID    string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		return "", errors.New("malformed payload")
	}
	if !strings.EqualFold(body.Algorithm, "HMAC-SHA256") || body.UserID == "" {
		return "", errors.New("unexpected payload")
	}
	return body.UserID, nil
}
