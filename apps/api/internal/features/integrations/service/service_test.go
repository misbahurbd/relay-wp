package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

func TestIsBlockedHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "127.0.0.1": true, "::1": true, "10.1.2.3": true, "172.16.0.1": true,
		"192.168.1.1": true, "169.254.169.254": true, "100.64.0.1": true, "0.0.0.0": true, "::ffff:127.0.0.1": true,
		"fd00::1": true, "224.0.0.1": true,
		"8.8.8.8": false, "2606:4700::1111": false, "hooks.example.com": false,
	} {
		if got := isBlockedHost(host); got != want {
			t.Errorf("isBlockedHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestSafeClientRefusesPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer srv.Close()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	resp, err := newSafeClient(false).Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("got %v, want the private address refused", err)
	}
	resp, err = newSafeClient(true).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

func TestCheckTargetURL(t *testing.T) {
	strict := &Service{}
	for raw, ok := range map[string]bool{
		"https://hooks.example.com/relay": true,
		"http://hooks.example.com":        false,
		"https://127.0.0.1/x":             false,
		"https://user:pw@example.com":     false,
		"ftp://example.com":               false,
		"/relative":                       false,
	} {
		if err := strict.checkTargetURL(raw); (err == nil) != ok {
			t.Errorf("checkTargetURL(%q) = %v, want ok=%v", raw, err, ok)
		}
	}
	dev := &Service{cfg: Config{AllowPrivateTargets: true}}
	if err := dev.checkTargetURL("http://localhost:4000/hook"); err != nil {
		t.Errorf("development should allow local http: %v", err)
	}
}

func TestNormalizeShop(t *testing.T) {
	for in, want := range map[string]string{
		"greenleaf":               "greenleaf.myshopify.com",
		"GreenLeaf.myshopify.com": "greenleaf.myshopify.com",
		"https://greenleaf.myshopify.com/admin/x": "greenleaf.myshopify.com",
		"evil.com":                         "",
		"greenleaf.myshopify.com.evil.com": "",
		"":                                 "",
	} {
		got, err := normalizeShop(in)
		if want == "" {
			if !errors.Is(err, model.ErrInvalidShop) {
				t.Errorf("normalizeShop(%q) = %q, want invalid", in, got)
			}
			continue
		}
		if got != want {
			t.Errorf("normalizeShop(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSignMatchesStripeScheme(t *testing.T) {
	// HMAC-SHA256("whsec_test", "1700000000.{}"), computed independently with Python's hmac.
	got := Sign("whsec_test", time.Unix(1700000000, 0), []byte("{}"))
	want := "t=1700000000,v1=35495024f4ef3f94e5a93e22221544c4b75e9a42300cd965ab81cb85cd994e91"
	if got != want {
		t.Fatalf("Sign = %s, want %s", got, want)
	}
}

func TestGoogleState(t *testing.T) {
	now := time.Unix(1700000000, 0)
	s := &Service{cfg: Config{SecretKey: make([]byte, 32), Google: GoogleConfig{ClientID: "id"}}, now: func() time.Time { return now }}
	authURL, nonce, err := s.AuthorizeGoogle(uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	state := u.Query().Get("state")
	if _, err := s.readState(state, model.ProviderGoogleSheets, nonce); err != nil {
		t.Fatalf("own state refused: %v", err)
	}
	if _, err := s.readState(state, model.ProviderGoogleSheets, "other-browser"); !errors.Is(err, model.ErrBadState) {
		t.Fatal("state accepted without its browser's nonce")
	}
	if _, err := s.readState(state, model.ProviderGoogleSheets, ""); !errors.Is(err, model.ErrBadState) {
		t.Fatal("state accepted without any nonce")
	}
	if _, err := s.readState(state, model.ProviderShopify, nonce); !errors.Is(err, model.ErrBadState) {
		t.Fatal("state accepted for another provider")
	}
	now = now.Add(16 * time.Minute)
	if _, err := s.readState(state, model.ProviderGoogleSheets, nonce); !errors.Is(err, model.ErrBadState) {
		t.Fatal("expired state accepted")
	}
}
