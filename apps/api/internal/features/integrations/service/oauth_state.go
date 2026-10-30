package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

// stateTTL is how long a person has to finish connecting at the provider.
const stateTTL = 15 * time.Minute

// oauthState is what a connect flow carries through the provider, HMAC-signed so
// no table is needed: who started it, for which app, and for which store.
type oauthState struct {
	WorkspaceID uuid.UUID `json:"w"`
	UserID      uuid.UUID `json:"u"`
	Provider    string    `json:"p"`
	// Target is the shop domain (Shopify) or store URL (WooCommerce) the person entered.
	Target  string `json:"t,omitempty"`
	Expires int64  `json:"e"`
	// NonceHash binds the state to the browser that started: its hash is here, the
	// nonce itself in a cookie only that browser has. Empty when the provider calls
	// back server to server (WooCommerce), where there is no browser.
	NonceHash string `json:"n,omitempty"`
}

// newState signs a state for the provider. With withNonce, it also returns the
// nonce the handler puts in a cookie.
func (s *Service) newState(workspaceID, userID uuid.UUID, provider, target string, withNonce bool) (state, nonce string, err error) {
	st := oauthState{
		WorkspaceID: workspaceID, UserID: userID, Provider: provider, Target: target,
		Expires: s.now().Add(stateTTL).Unix(),
	}
	if withNonce {
		if nonce, err = randomToken(24); err != nil {
			return "", "", err
		}
		st.NonceHash = base64URL.EncodeToString(hashKey(nonce))
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return "", "", fmt.Errorf("marshal state: %w", err)
	}
	payload := base64URL.EncodeToString(raw)
	return payload + "." + base64URL.EncodeToString(s.stateMAC(payload)), nonce, nil
}

func (s *Service) stateMAC(payload string) []byte {
	// A key derived for this use only, so the encryption key itself never signs anything.
	k := hmac.New(sha256.New, s.cfg.SecretKey)
	k.Write([]byte("integration-oauth-state"))
	mac := hmac.New(sha256.New, k.Sum(nil))
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// readState checks the signature, expiry and provider. A state made with a nonce
// also needs that nonce back (from the cookie); one made without needs none.
func (s *Service) readState(state, provider, nonce string) (oauthState, error) {
	payload, sig, ok := strings.Cut(state, ".")
	if !ok {
		return oauthState{}, model.ErrBadState
	}
	got, err := base64URL.DecodeString(sig)
	if err != nil || !hmac.Equal(got, s.stateMAC(payload)) {
		return oauthState{}, model.ErrBadState
	}
	raw, err := base64URL.DecodeString(payload)
	if err != nil {
		return oauthState{}, model.ErrBadState
	}
	var st oauthState
	if err := json.Unmarshal(raw, &st); err != nil || s.now().Unix() > st.Expires || st.Provider != provider {
		return oauthState{}, model.ErrBadState
	}
	if st.NonceHash != "" && (nonce == "" ||
		!hmac.Equal([]byte(st.NonceHash), []byte(base64URL.EncodeToString(hashKey(nonce))))) {
		return oauthState{}, model.ErrBadState
	}
	return st, nil
}

// connectFailure is where a browser callback sends the person when connecting
// fails: Integrations with ?integration_error=<code>. Unexpected errors are logged.
func (s *Service) connectFailure(back, provider string, err error, fallback string) string {
	code := fallback
	if ae := asAppErr(err); ae != nil {
		code = ae.Code
	} else if s.log != nil {
		s.log.Warn("integration connect failed", "provider", provider, "err", err)
	}
	return back + "?integration_error=" + code
}

func (s *Service) integrationsPage() string {
	return strings.TrimRight(s.cfg.AppBaseURL, "/") + "/integrations"
}
