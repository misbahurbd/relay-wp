package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth"
)

const (
	testAPI         = "http://api.test"
	testAppRedirect = "relay://auth/callback"
	googleClientID  = "google-test-client"
	fbAppSecret     = "fb-test-secret"
)

// --- fake provider ---

// profile is what the fake provider says about the person who "approves" the sign-in.
type profile struct {
	Subject       string
	Email         string
	EmailVerified bool
	FirstName     string
	LastName      string
	Picture       string
	Silhouette    bool   // Facebook default picture
	WrongNonce    bool   // Google returns an ID token for a different nonce
	nonce         string // filled from the authorization URL
	challenge     string
}

// fakeProvider plays Google (OIDC: token endpoint, signed ID tokens, JWKS) and
// Facebook (token endpoint, Graph /me with appsecret_proof) on one httptest server.
type fakeProvider struct {
	t   *testing.T
	srv *httptest.Server
	key *rsa.PrivateKey

	mu    sync.Mutex
	codes map[string]profile // authorization code → who approved it
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeProvider{t: t, key: key, codes: map[string]profile{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /google/jwks", f.jwks)
	mux.HandleFunc("POST /google/token", f.googleToken)
	mux.HandleFunc("POST /fb/token", f.fbToken)
	mux.HandleFunc("GET /fb/me", f.fbMe)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeProvider) config() *auth.ProvidersConfig {
	return &auth.ProvidersConfig{
		Google: auth.GoogleConfig{
			ClientID: googleClientID, ClientSecret: "google-test-secret",
			AuthURL: f.srv.URL + "/google/auth", TokenURL: f.srv.URL + "/google/token",
			JWKSURL: f.srv.URL + "/google/jwks", Issuer: f.srv.URL,
		},
		Facebook: auth.FacebookConfig{
			AppID: "fb-test-app", AppSecret: fbAppSecret,
			AuthURL: f.srv.URL + "/fb/auth", TokenURL: f.srv.URL + "/fb/token", GraphURL: f.srv.URL + "/fb",
		},
	}
}

// approve simulates the person signing in at the provider: it returns the code
// the provider would send back for this authorization URL.
func (f *fakeProvider) approve(authURL string, p profile) string {
	f.t.Helper()
	u, err := url.Parse(authURL)
	if err != nil || !strings.HasPrefix(authURL, f.srv.URL) {
		f.t.Fatalf("start redirected to %q, not the fake provider", authURL)
	}
	p.nonce, p.challenge = u.Query().Get("nonce"), u.Query().Get("code_challenge")
	code := uuid.NewString()
	f.mu.Lock()
	f.codes[code] = p
	f.mu.Unlock()
	return code
}

func (f *fakeProvider) take(code string) (profile, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.codes[code]
	delete(f.codes, code)
	return p, ok
}

func (f *fakeProvider) googleToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	p, ok := f.take(r.Form.Get("code"))
	if !ok || r.Form.Get("client_id") != googleClientID {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant"})
		return
	}
	if oauth2.S256ChallengeFromVerifier(r.Form.Get("code_verifier")) != p.challenge {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid_grant", "error_description": "PKCE"})
		return
	}
	nonce := p.nonce
	if p.WrongNonce {
		nonce = "someone-elses-nonce"
	}
	now := time.Now()
	tok := gojwt.NewWithClaims(gojwt.SigningMethodRS256, gojwt.MapClaims{
		"iss": f.srv.URL, "aud": googleClientID, "sub": p.Subject, "nonce": nonce,
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"email": p.Email, "email_verified": p.EmailVerified,
		"given_name": p.FirstName, "family_name": p.LastName, "picture": p.Picture,
	})
	tok.Header["kid"] = "test-key"
	idToken, err := tok.SignedString(f.key)
	if err != nil {
		f.t.Error(err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": "google-access", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken,
	})
}

func (f *fakeProvider) jwks(w http.ResponseWriter, _ *http.Request) {
	b64 := base64.RawURLEncoding.EncodeToString
	writeJSON(w, http.StatusOK, map[string]any{"keys": []map[string]any{{
		"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256",
		"n": b64(f.key.N.Bytes()), "e": b64(big.NewInt(int64(f.key.E)).Bytes()),
	}}})
}

func (f *fakeProvider) fbToken(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	code := r.Form.Get("code")
	f.mu.Lock()
	_, ok := f.codes[code]
	f.mu.Unlock()
	if !ok || r.Form.Get("client_secret") != fbAppSecret {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "bad code"}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_token": "fb-" + code, "token_type": "bearer", "expires_in": 5183944})
}

func (f *fakeProvider) fbMe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("access_token")
	mac := hmac.New(sha256.New, []byte(fbAppSecret))
	mac.Write([]byte(token))
	if r.URL.Query().Get("appsecret_proof") != hex.EncodeToString(mac.Sum(nil)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "bad appsecret_proof"}})
		return
	}
	p, ok := f.take(strings.TrimPrefix(token, "fb-"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "bad token"}})
		return
	}
	me := map[string]any{
		"id": p.Subject, "first_name": p.FirstName, "last_name": p.LastName,
		"picture": map[string]any{"data": map[string]any{"url": p.Picture, "is_silhouette": p.Silhouette}},
	}
	if p.Email != "" {
		me["email"] = p.Email
	}
	writeJSON(w, http.StatusOK, me)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- client helpers ---

// oauthResult is where the callback sent the browser.
type oauthResult struct {
	resp
	location *url.URL
}

func (o oauthResult) param(k string) string { return o.location.Query().Get(k) }

// startOAuth calls /start (or /link) and returns the provider URL it redirects to.
func (b *browser) startOAuth(path string) string {
	b.env.t.Helper()
	r := b.do("GET", path, nil)
	expect(b.env.t, r, http.StatusFound)
	if b.cookies["relay_oauth"] == nil {
		b.env.t.Fatal("start did not set the relay_oauth cookie")
	}
	return r.header.Get("Location")
}

// callback returns from the provider with code and state, as the browser would.
func (b *browser) callback(provider, providerURL, code string) oauthResult {
	b.env.t.Helper()
	u, _ := url.Parse(providerURL)
	q := url.Values{"code": {code}, "state": {u.Query().Get("state")}}
	r := b.do("GET", "/v1/auth/oauth/"+provider+"/callback?"+q.Encode(), nil)
	expect(b.env.t, r, http.StatusFound)
	loc, err := url.Parse(r.header.Get("Location"))
	if err != nil {
		b.env.t.Fatal(err)
	}
	return oauthResult{resp: r, location: loc}
}

// social runs a whole web sign-in with provider as person p.
func (b *browser) social(provider string, p profile) oauthResult {
	b.env.t.Helper()
	authURL := b.startOAuth("/v1/auth/oauth/" + provider + "/start?redirect_to=" + url.QueryEscape(testOrigin+"/dashboard"))
	return b.callback(provider, authURL, b.env.fake.approve(authURL, p))
}

func (e *authEnv) me(b *browser) map[string]any {
	e.t.Helper()
	r := b.do("GET", "/v1/me", nil)
	expect(e.t, r, http.StatusOK)
	return r.body
}

func field(m map[string]any, path ...string) any {
	var v any = m
	for _, p := range path {
		v = v.(map[string]any)[p]
	}
	return v
}

func (e *authEnv) identities(b *browser) (bool, []string) {
	e.t.Helper()
	r := b.do("GET", "/v1/auth/identities", nil)
	expect(e.t, r, http.StatusOK)
	var names []string
	for _, i := range r.body["identities"].([]any) {
		names = append(names, i.(map[string]any)["provider"].(string))
	}
	return r.body["has_password"].(bool), names
}

func (e *authEnv) exec(sql string, args ...any) {
	e.t.Helper()
	if _, err := e.a.db.Exec(context.Background(), sql, args...); err != nil {
		e.t.Fatal(err)
	}
}

func googleUser(email string) profile {
	return profile{
		Subject: "g-" + uuid.NewString(), Email: email, EmailVerified: true,
		FirstName: "Grace", LastName: "Hopper", Picture: "https://lh3.googleusercontent.com/a/grace",
	}
}

func facebookUser(email string) profile {
	return profile{
		Subject: "fb-" + uuid.NewString()[:12], Email: email,
		FirstName: "Linus", LastName: "Torvalds", Picture: "https://scontent.xx.fbcdn.net/linus.jpg",
	}
}

// --- tests ---

func TestProvidersList(t *testing.T) {
	e := newAuthEnv(t)
	r := e.browser(chromeMac).do("GET", "/v1/auth/providers", nil)
	expect(t, r, http.StatusOK)
	if got := r.raw; !strings.Contains(got, `"google"`) || !strings.Contains(got, `"facebook"`) || strings.Contains(got, "apple") {
		t.Errorf("providers = %s", got)
	}
	expect(t, e.browser(chromeMac).do("GET", "/v1/auth/oauth/apple/start", nil), http.StatusNotFound, "provider_not_available")
}

func TestGoogleSignUpAndSignIn(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	person := googleUser(email)

	b := e.browser(chromeMac)
	res := b.social("google", person)
	if res.location.String() != testOrigin+"/dashboard" {
		t.Fatalf("landed on %s", res.location)
	}
	if b.cookies["relay_access"] == nil || b.cookies["relay_refresh"] == nil {
		t.Fatal("web sign-in must set auth cookies")
	}
	if c := b.cookies["relay_oauth"]; c != nil {
		t.Error("callback must clear the relay_oauth cookie")
	}
	me := e.me(b)
	if field(me, "user", "email_verified") != true || field(me, "user", "has_password") != false ||
		field(me, "profile", "first_name") != "Grace" || field(me, "profile", "avatar_url") != person.Picture {
		t.Errorf("/me = %v", me)
	}
	userID := field(me, "user", "id")

	// Signing in again finds the same user and never overwrites the profile.
	person.FirstName = "Changed"
	b2 := e.browser(safariPhone)
	b2.social("google", person)
	me2 := e.me(b2)
	if field(me2, "user", "id") != userID || field(me2, "profile", "first_name") != "Grace" {
		t.Errorf("second sign-in /me = %v", me2)
	}
	sessions := b2.do("GET", "/v1/auth/sessions", nil)
	if !strings.Contains(sessions.raw, `"provider":"google"`) {
		t.Errorf("sessions = %s", sessions.raw)
	}
	// A social-only account has no password to sign in with.
	expect(t, b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}), http.StatusUnauthorized)
}

func TestGoogleLinksVerifiedPasswordAccount(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	pw := e.browser(chromeMac)
	pw.login(email)
	userID := field(e.me(pw), "user", "id")

	g := e.browser(chromeMac)
	g.social("google", googleUser(email))
	if field(e.me(g), "user", "id") != userID {
		t.Fatal("Google sign-in with the same verified email must reach the existing account")
	}
	hasPassword, ids := e.identities(g)
	if !hasPassword || len(ids) != 1 || ids[0] != "google" {
		t.Errorf("identities: has_password=%v %v", hasPassword, ids)
	}
	expect(t, pw.do("GET", "/v1/me", nil), http.StatusOK) // existing sessions keep working
}

func TestGoogleClaimsUnverifiedAccount(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	// Someone signs up with this email but never verifies it.
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Squatter",
	}), http.StatusAccepted)

	// The real owner proves the email through Google.
	owner := e.browser(chromeMac)
	owner.social("google", googleUser(email))
	me := e.me(owner)
	if field(me, "user", "email_verified") != true || field(me, "user", "has_password") != false {
		t.Errorf("/me = %v", me)
	}
	// The squatter's password is gone.
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusUnauthorized, "invalid_credentials")
}

func TestUnverifiedProviderEmailNeverLinks(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()

	fb := e.browser(chromeMac).social("facebook", facebookUser(email))
	if fb.param("error") != "email_in_use_sign_in_first" || fb.location.Path != "/dashboard" {
		t.Errorf("facebook with a taken email landed on %s", fb.location)
	}
	unverified := googleUser(email)
	unverified.EmailVerified = false
	g := e.browser(chromeMac).social("google", unverified)
	if g.param("error") != "email_in_use_sign_in_first" {
		t.Errorf("unverified google email landed on %s", g.location)
	}
	for _, c := range g.cookies {
		if c.Name == "relay_access" && c.MaxAge > 0 {
			t.Error("a refused sign-in must not set auth cookies")
		}
	}
}

func TestFacebookSignUp(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	person := facebookUser(email)
	person.Silhouette = true

	b := e.browser(chromeMac)
	res := b.social("facebook", person)
	if res.param("error") != "" {
		t.Fatalf("landed on %s", res.location)
	}
	me := e.me(b)
	// Facebook never vouches for email: the account is unverified and gets a link.
	if field(me, "user", "email_verified") != false || field(me, "profile", "avatar_url") != nil ||
		field(me, "profile", "first_name") != "Linus" {
		t.Errorf("/me = %v", me)
	}
	if subject, _ := e.lastEmail(email); subject != "Verify your email address" {
		t.Errorf("email subject = %q", subject)
	}
}

func TestFacebookWithoutEmail(t *testing.T) {
	e := newAuthEnv(t)
	person := facebookUser("")

	b := e.browser(chromeMac)
	res := b.social("facebook", person)
	token := res.param("complete_token")
	if token == "" || b.cookies["relay_access"] != nil {
		t.Fatalf("expected the add-email step, landed on %s", res.location)
	}

	taken := e.verifiedUser()
	expect(t, b.do("POST", "/v1/auth/oauth/complete", map[string]any{"token": token, "email": taken}),
		http.StatusConflict, "email_in_use_sign_in_first")

	email := newEmail()
	expect(t, b.do("POST", "/v1/auth/oauth/complete", map[string]any{"token": token, "email": email}), http.StatusOK)
	if b.cookies["relay_access"] == nil {
		t.Fatal("completing the sign-in must set auth cookies")
	}
	if field(e.me(b), "user", "email") != email {
		t.Error("account must use the typed email")
	}
	if subject, _ := e.lastEmail(email); subject != "Verify your email address" {
		t.Errorf("email subject = %q", subject)
	}
	expect(t, b.do("POST", "/v1/auth/oauth/complete", map[string]any{"token": token, "email": newEmail()}),
		http.StatusUnprocessableEntity, "invalid_oauth_state")

	// Next time, the same Facebook account signs straight in.
	again := e.browser(chromeMac)
	if r := again.social("facebook", person); r.param("complete_token") != "" || again.cookies["relay_access"] == nil {
		t.Errorf("second facebook sign-in landed on %s", r.location)
	}
}

func TestOAuthStateChecks(t *testing.T) {
	e := newAuthEnv(t)
	person := googleUser(newEmail())

	t.Run("callback from another browser", func(t *testing.T) {
		victim, attacker := e.browser(chromeMac), e.browser(chromeMac)
		authURL := attacker.startOAuth("/v1/auth/oauth/google/start")
		code := e.fake.approve(authURL, person)
		// The attacker sends the victim their own callback link.
		res := victim.callback("google", authURL, code)
		if res.param("error") != "invalid_oauth_state" || victim.cookies["relay_access"] != nil {
			t.Errorf("landed on %s", res.location)
		}
	})
	t.Run("replayed callback", func(t *testing.T) {
		b := e.browser(chromeMac)
		authURL := b.startOAuth("/v1/auth/oauth/google/start")
		state := b.cookies["relay_oauth"]
		b.callback("google", authURL, e.fake.approve(authURL, person))
		b.cookies["relay_oauth"] = state
		if res := b.callback("google", authURL, "any"); res.param("error") != "invalid_oauth_state" {
			t.Errorf("replay landed on %s", res.location)
		}
	})
	t.Run("person cancels at the provider", func(t *testing.T) {
		b := e.browser(chromeMac)
		authURL := b.startOAuth("/v1/auth/oauth/google/start")
		u, _ := url.Parse(authURL)
		r := b.do("GET", "/v1/auth/oauth/google/callback?error=access_denied&state="+u.Query().Get("state"), nil)
		if loc := r.header.Get("Location"); !strings.Contains(loc, "error=oauth_cancelled") {
			t.Errorf("landed on %s", loc)
		}
	})
	t.Run("ID token for another nonce", func(t *testing.T) {
		p := person
		p.WrongNonce = true
		if res := e.browser(chromeMac).social("google", p); res.param("error") != "oauth_failed" {
			t.Errorf("landed on %s", res.location)
		}
	})
	t.Run("foreign redirect_to falls back to the app", func(t *testing.T) {
		b := e.browser(chromeMac)
		authURL := b.startOAuth("/v1/auth/oauth/google/start?redirect_to=" + url.QueryEscape("https://evil.test/steal"))
		res := b.callback("google", authURL, e.fake.approve(authURL, person))
		if res.location.Host != "app.test" {
			t.Errorf("landed on %s", res.location)
		}
	})
}

func TestMobileOAuth(t *testing.T) {
	e := newAuthEnv(t)
	person := googleUser(newEmail())
	app := e.browser("RelayApp/1.0 okhttp/4.12")
	app.device = "Pixel 9"
	verifier := oauth2.GenerateVerifier()
	start := "/v1/auth/oauth/google/start?client=mobile&redirect_to=" + url.QueryEscape(testAppRedirect) +
		"&code_challenge=" + oauth2.S256ChallengeFromVerifier(verifier)

	expect(t, app.do("GET", "/v1/auth/oauth/google/start?client=mobile&redirect_to="+url.QueryEscape(testAppRedirect), nil),
		http.StatusUnprocessableEntity, "invalid_input")
	expect(t, app.do("GET", "/v1/auth/oauth/google/start?client=mobile&redirect_to=evil://x&code_challenge="+
		oauth2.S256ChallengeFromVerifier(verifier), nil), http.StatusUnprocessableEntity, "invalid_redirect")

	signIn := func() string {
		authURL := app.startOAuth(start)
		res := app.callback("google", authURL, e.fake.approve(authURL, person))
		if !strings.HasPrefix(res.location.String(), testAppRedirect+"?") || res.param("code") == "" {
			t.Fatalf("landed on %s", res.location)
		}
		if app.cookies["relay_access"] != nil {
			t.Fatal("mobile sign-in must not set auth cookies")
		}
		return res.param("code")
	}

	// A wrong verifier spends the code.
	code := signIn()
	expect(t, app.do("POST", "/v1/auth/oauth/token", map[string]any{"code": code, "code_verifier": oauth2.GenerateVerifier()}),
		http.StatusUnauthorized, "invalid_code")
	expect(t, app.do("POST", "/v1/auth/oauth/token", map[string]any{"code": code, "code_verifier": verifier}),
		http.StatusUnauthorized, "invalid_code")

	code = signIn()
	r := app.do("POST", "/v1/auth/oauth/token", map[string]any{"code": code, "code_verifier": verifier})
	expect(t, r, http.StatusOK)
	app.bearer = r.body["access_token"].(string)
	if r.body["refresh_token"] == nil {
		t.Error("mobile gets the refresh token in the body")
	}
	sessions := app.do("GET", "/v1/auth/sessions", nil)
	if !strings.Contains(sessions.raw, "Relay app on Pixel 9") {
		t.Errorf("sessions = %s", sessions.raw)
	}
	expect(t, app.do("POST", "/v1/auth/oauth/token", map[string]any{"code": code, "code_verifier": verifier}),
		http.StatusUnauthorized, "invalid_code")
}

func TestLinkAndUnlink(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)
	person := googleUser(newEmail()) // a Google account with a different email
	link := "/v1/auth/oauth/google/link?redirect_to=" + url.QueryEscape(testOrigin+"/settings")

	// Connecting needs a recent sign-in.
	e.exec(`UPDATE auth_sessions SET authenticated_at = now() - interval '1 hour' WHERE user_id = (SELECT id FROM users WHERE email = $1)`, email)
	expect(t, b.do("GET", link, nil), http.StatusForbidden, "reauthentication_required")
	b.login(email)

	authURL := b.startOAuth(link)
	res := b.callback("google", authURL, e.fake.approve(authURL, person))
	if res.param("linked") != "google" || res.location.Path != "/settings" {
		t.Fatalf("landed on %s", res.location)
	}
	if hasPassword, ids := e.identities(b); !hasPassword || len(ids) != 1 {
		t.Errorf("identities: %v %v", hasPassword, ids)
	}
	// That Google account now signs in to this user.
	g := e.browser(chromeMac)
	g.social("google", person)
	if field(e.me(g), "user", "email") != email {
		t.Error("linked Google account must sign in to the linking user")
	}

	// Another user can't take it.
	other := e.browser(chromeMac)
	other.login(e.verifiedUser())
	authURL = other.startOAuth(link)
	if res := other.callback("google", authURL, e.fake.approve(authURL, person)); res.param("error") != "identity_in_use" {
		t.Errorf("landed on %s", res.location)
	}

	expect(t, b.do("DELETE", "/v1/auth/identities/google", nil), http.StatusNoContent)
	expect(t, b.do("DELETE", "/v1/auth/identities/google", nil), http.StatusNotFound, "identity_not_found")

	// A Google-only account can't remove its only way in.
	only := e.browser(chromeMac)
	only.social("google", googleUser(newEmail()))
	expect(t, only.do("DELETE", "/v1/auth/identities/google", nil), http.StatusConflict, "last_sign_in_method")
}

func TestSetPasswordForSocialUser(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	b := e.browser(chromeMac)
	b.social("google", googleUser(email))

	expect(t, b.do("POST", "/v1/auth/password/set", map[string]any{"new_password": "weak"}), http.StatusUnprocessableEntity, "weak_password")
	expect(t, b.do("POST", "/v1/auth/password/set", map[string]any{"new_password": testPassword}), http.StatusNoContent)
	expect(t, b.do("POST", "/v1/auth/password/set", map[string]any{"new_password": testPassword}), http.StatusConflict, "password_already_set")
	e.browser(chromeMac).login(email)
	if hasPassword, _ := e.identities(b); !hasPassword {
		t.Error("has_password must be true")
	}

	// Facebook accounts must verify their email first.
	fb := e.browser(chromeMac)
	fb.social("facebook", facebookUser(newEmail()))
	expect(t, fb.do("POST", "/v1/auth/password/set", map[string]any{"new_password": testPassword}), http.StatusForbidden, "email_not_verified")

	// And it needs a recent sign-in.
	stale := e.browser(chromeMac)
	stale.social("google", googleUser(newEmail()))
	e.exec(`UPDATE auth_sessions SET authenticated_at = now() - interval '1 hour'`)
	expect(t, stale.do("POST", "/v1/auth/password/set", map[string]any{"new_password": testPassword}), http.StatusForbidden, "reauthentication_required")
}

func TestPasswordResetDropsUnverifiedFacebook(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	person := facebookUser(email)
	e.browser(chromeMac).social("facebook", person)

	// The real owner of the email resets the password: the Facebook link goes.
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/password/forgot", map[string]any{"email": email}), http.StatusAccepted)
	_, token := e.lastEmail(email)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/password/reset", map[string]any{"token": token, "new_password": testPassword}), http.StatusNoContent)

	owner := e.browser(chromeMac)
	owner.login(email)
	if _, ids := e.identities(owner); len(ids) != 0 {
		t.Errorf("identities after reset = %v", ids)
	}
	if res := e.browser(chromeMac).social("facebook", person); res.param("error") != "email_in_use_sign_in_first" {
		t.Errorf("old facebook sign-in landed on %s", res.location)
	}
}

func TestDisabledUserCannotUseSocialSignIn(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	person := googleUser(email)
	e.browser(chromeMac).social("google", person)
	e.exec(`UPDATE users SET status = 'disabled' WHERE email = $1`, email)

	b := e.browser(chromeMac)
	if res := b.social("google", person); res.param("error") != "account_disabled" || b.cookies["relay_access"] != nil {
		t.Errorf("landed on %s", res.location)
	}
}

func TestFacebookDataDeletion(t *testing.T) {
	e := newAuthEnv(t)
	person := facebookUser(newEmail())
	b := e.browser(chromeMac)
	b.social("facebook", person)

	signed := func(secret string) string {
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"algorithm":"HMAC-SHA256","issued_at":1730000000,"user_id":"` + person.Subject + `"}`))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) + "." + payload
	}
	post := func(body string) resp {
		req := httptest.NewRequestWithContext(t.Context(), "POST", "/v1/auth/oauth/facebook/data-deletion", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = "10.9.9.9:1"
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		out := resp{code: rec.Code, raw: rec.Body.String()}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
		return out
	}

	expect(t, post("signed_request="+url.QueryEscape(signed("wrong-secret"))), http.StatusUnprocessableEntity, "invalid_signed_request")
	r := post("signed_request=" + url.QueryEscape(signed(fbAppSecret)))
	expect(t, r, http.StatusOK)
	code, _ := r.body["confirmation_code"].(string)
	if code == "" || r.body["url"] != testOrigin+"/data-deletion?code="+code {
		t.Errorf("response = %s", r.raw)
	}
	if _, ids := e.identities(b); len(ids) != 0 {
		t.Errorf("identities after deletion = %v", ids)
	}
	events := b.do("GET", "/v1/auth/events", nil)
	if !strings.Contains(events.raw, `"event":"identity_unlinked"`) {
		t.Errorf("events = %s", events.raw)
	}
}
