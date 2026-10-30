package service

import (
	"context"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

// WooCallbackPath is where a store posts the REST keys it made, under API_BASE_URL.
const WooCallbackPath = "/v1/integrations/woocommerce/callback"

// WooOAuthAvailable reports whether stores can connect through WooCommerce's
// authorize page: it posts the keys to our callback, which it only does over https.
func (s *Service) WooOAuthAvailable() bool {
	return strings.HasPrefix(s.cfg.APIBaseURL, "https://")
}

// AuthorizeWooCommerce returns the store's "Connect Relay" page (WooCommerce's
// /wc-auth/v1/authorize). Approving it makes read-only REST keys and posts them to
// WooCallback; the store then sends the person back to Integrations.
func (s *Service) AuthorizeWooCommerce(workspaceID, userID uuid.UUID, storeURL string) (string, error) {
	if !s.WooOAuthAvailable() {
		return "", model.ErrWooOAuthUnavailable
	}
	store, err := s.normalizeStoreURL(storeURL)
	if err != nil {
		return "", err
	}
	// No nonce: the store calls back server to server, so no browser cookie comes with it.
	state, _, err := s.newState(workspaceID, userID, model.ProviderWooCommerce, store, false)
	if err != nil {
		return "", err
	}
	q := url.Values{
		"app_name": {"Relay"}, "scope": {"read"}, "user_id": {state},
		"return_url":   {s.integrationsPage() + "?provider=" + model.ProviderWooCommerce},
		"callback_url": {strings.TrimRight(s.cfg.APIBaseURL, "/") + WooCallbackPath},
	}
	return store + "/wc-auth/v1/authorize?" + q.Encode(), nil
}

// WooCallback saves the keys a store posted. state is the signed user_id we sent;
// the keys are checked against the store named in it, so a forged post can't
// attach another store. An error makes WooCommerce show the person a failure.
func (s *Service) WooCallback(ctx context.Context, state, consumerKey, consumerSecret string) error {
	st, err := s.readState(state, model.ProviderWooCommerce, "")
	if err != nil {
		return err
	}
	creds := storeCredentials{ConsumerKey: strings.TrimSpace(consumerKey), ConsumerSecret: strings.TrimSpace(consumerSecret)}
	if creds.ConsumerKey == "" || creds.ConsumerSecret == "" {
		return model.ErrMissingFields
	}
	label, err := s.checkWooCommerce(ctx, st.Target, creds)
	if err != nil {
		return err
	}
	return s.saveConnection(ctx, st.WorkspaceID, st.UserID, model.ProviderWooCommerce, label,
		storeConfig{StoreURL: st.Target}, creds)
}
