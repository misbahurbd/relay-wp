package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/jwt"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/queue"
)

const (
	testOrigin   = "http://app.test"
	testPassword = "Str0ng!pass"
	chromeMac    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36"
	safariPhone  = "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1"
)

// --- test harness ---

type authEnv struct {
	t    *testing.T
	a    *App
	h    http.Handler
	fake *fakeProvider
}

func newAuthEnv(t *testing.T) *authEnv {
	t.Helper()
	pool := dbtest.New(t)
	seed, _, err := jwt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	q, err := queue.New(pool, nil, 0, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakeProvider(t)
	a := &App{
		cfg: config.Config{
			AppBaseURL: testOrigin,
			HTTP: config.HTTP{
				CORSAllowedOrigins: []string{testOrigin},
				RateLimitRequests:  1000,
				RateLimitWindow:    time.Minute,
			},
			Auth: config.Auth{
				JWTSigningKey: seed, JWTSigningKeyID: "test", JWTIssuer: "relay-api", JWTAudience: "relay",
				AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 720 * time.Hour, CookieSecure: true,
				APIBaseURL: testAPI, AllowedRedirects: []string{testAppRedirect}, ReauthMaxAge: 10 * time.Minute,
			},
		},
		log: slog.Default(), db: pool, queue: q, oauth: fake.config(),
	}
	if err := a.buildFeatures(); err != nil {
		t.Fatal(err)
	}
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &authEnv{t: t, a: a, h: h, fake: fake}
}

// browser is one client: it keeps cookies like a browser, or a bearer token like the app.
type browser struct {
	env     *authEnv
	ip      string
	ua      string
	cookies map[string]*http.Cookie
	bearer  string
	device  string // X-Device-Name, as the mobile app sends it
}

func (e *authEnv) browser(ua string) *browser {
	return &browser{
		env: e, ua: ua, cookies: map[string]*http.Cookie{},
		ip: fmt.Sprintf("10.%d.%d.%d", rand.IntN(250), rand.IntN(250), rand.IntN(250)+1), //nolint:gosec // test data
	}
}

type resp struct {
	code    int
	body    map[string]any
	raw     string
	cookies []*http.Cookie
	header  http.Header
}

func (b *browser) do(method, path string, body any) resp {
	b.env.t.Helper()
	var r io.Reader
	if body != nil {
		buf, _ := json.Marshal(body)
		r = bytes.NewReader(buf)
	}
	req := httptest.NewRequestWithContext(b.env.t.Context(), method, path, r)
	req.RemoteAddr = b.ip + ":4321"
	req.Header.Set("User-Agent", b.ua)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	if b.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+b.bearer)
	}
	if b.device != "" {
		req.Header.Set("X-Device-Name", b.device)
	}
	for _, c := range b.cookies {
		if strings.HasPrefix(path, c.Path) {
			req.AddCookie(c)
		}
	}

	rec := httptest.NewRecorder()
	b.env.h.ServeHTTP(rec, req)

	out := resp{code: rec.Code, raw: rec.Body.String(), cookies: rec.Result().Cookies(), header: rec.Header()}
	_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
	for _, c := range out.cookies {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return out
}

func expect(t *testing.T, r resp, code int, errCode ...string) {
	t.Helper()
	if r.code != code {
		t.Fatalf("status %d, want %d; body: %s", r.code, code, r.raw)
	}
	if len(errCode) > 0 && r.body["code"] != errCode[0] {
		t.Fatalf("error code %v, want %s; body: %s", r.body["code"], errCode[0], r.raw)
	}
}

var tokenRe = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)

// lastEmail returns the subject and link token of the newest queued email to addr.
func (e *authEnv) lastEmail(addr string) (subject, token string) {
	e.t.Helper()
	var args []byte
	err := e.a.db.QueryRow(context.Background(),
		`SELECT args FROM river_job WHERE kind = 'send_email' AND args->'message'->'to' ? $1 ORDER BY id DESC LIMIT 1`,
		addr).Scan(&args)
	if err != nil {
		e.t.Fatalf("no email queued for %s: %v", addr, err)
	}
	var job struct {
		Message struct{ Subject, Text string } `json:"message"`
	}
	if err := json.Unmarshal(args, &job); err != nil {
		e.t.Fatal(err)
	}
	if m := tokenRe.FindStringSubmatch(job.Message.Text); m != nil {
		token = m[1]
	}
	return job.Message.Subject, token
}

func newEmail() string { return "user-" + uuid.NewString()[:8] + "@relay.test" }

// verifiedUser signs up and verifies a new account, returning its email.
func (e *authEnv) verifiedUser() string {
	e.t.Helper()
	email := newEmail()
	b := e.browser(chromeMac)
	expect(e.t, b.do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Ada", "last_name": "Lovelace", "timezone": "Asia/Dhaka",
	}), http.StatusAccepted)
	_, token := e.lastEmail(email)
	expect(e.t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token}), http.StatusNoContent)
	return email
}

// login signs in as the web app (cookies). Mobile sign-in is tested directly in TestMobileTokens.
func (b *browser) login(email string) resp {
	b.env.t.Helper()
	r := b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword})
	expect(b.env.t, r, http.StatusOK)
	return r
}

// --- flows ---

func TestSignupVerifyLoginMe(t *testing.T) {
	e := newAuthEnv(t)
	b := e.browser(chromeMac)
	email := newEmail()

	expect(t, b.do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Ada", "last_name": "Lovelace", "timezone": "Asia/Dhaka",
	}), http.StatusAccepted)

	// Correct password but unverified: told to verify.
	expect(t, b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusForbidden, "email_not_verified")

	subject, token := e.lastEmail(email)
	if subject != "Verify your email address" || token == "" {
		t.Fatalf("verification email: subject=%q token=%q", subject, token)
	}
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token}), http.StatusNoContent)
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token}), http.StatusUnprocessableEntity, "invalid_link")

	r := b.login(email)
	if r.body["access_token"] != nil || r.body["refresh_token"] != nil {
		t.Error("web login must not return tokens in the body")
	}
	access, refresh := b.cookies["relay_access"], b.cookies["relay_refresh"]
	if access == nil || refresh == nil {
		t.Fatalf("missing auth cookies: %v", r.cookies)
	}
	for _, c := range []*http.Cookie{access, refresh} {
		if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
			t.Errorf("cookie %s: HttpOnly=%v Secure=%v SameSite=%v", c.Name, c.HttpOnly, c.Secure, c.SameSite)
		}
	}
	if refresh.Path != "/v1/auth/token/refresh" {
		t.Errorf("refresh cookie path = %q", refresh.Path)
	}

	me := b.do("GET", "/v1/me", nil)
	expect(t, me, http.StatusOK)
	user, profile, prefs := me.body["user"].(map[string]any), me.body["profile"].(map[string]any), me.body["preferences"].(map[string]any)
	if user["email"] != email || user["email_verified"] != true || user["has_password"] != true {
		t.Errorf("user = %v", user)
	}
	if profile["first_name"] != "Ada" || profile["last_name"] != "Lovelace" || profile["display_name"] != nil {
		t.Errorf("profile = %v", profile)
	}
	if prefs["timezone"] != "Asia/Dhaka" || prefs["locale"] != "en" {
		t.Errorf("preferences = %v", prefs)
	}
	if strings.Contains(strings.ToLower(me.raw), "hash") || strings.Contains(me.raw, "argon2") {
		t.Errorf("/me leaked a secret: %s", me.raw)
	}
}

func TestWebRefreshRotationAndReuse(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)

	first := b.cookies["relay_refresh"].Value
	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	second := b.cookies["relay_refresh"].Value
	if first == second {
		t.Fatal("refresh token was not rotated")
	}

	// Replaying the first (already rotated) token = theft: the whole session ends.
	attacker := e.browser(chromeMac)
	attacker.cookies["relay_refresh"] = &http.Cookie{Name: "relay_refresh", Value: first, Path: "/v1/auth/token/refresh"} //nolint:gosec // test client
	expect(t, attacker.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")

	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
	// The session is gone, so its still-unexpired access token stops working too.
	expect(t, b.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestRefreshCookieNeedsAllowedOrigin(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)

	req := httptest.NewRequestWithContext(t.Context(), "POST", "/v1/auth/token/refresh", nil)
	req.RemoteAddr = b.ip + ":1"
	req.Header.Set("Origin", "https://evil.test")
	req.AddCookie(b.cookies["relay_refresh"])
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site refresh: status %d, want 403", rec.Code)
	}
}

func TestMobileTokens(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	app := e.browser("okhttp/4.12")

	r := app.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword, "client": "mobile"})
	expect(t, r, http.StatusOK)
	if len(r.cookies) != 0 {
		t.Error("mobile login must not set cookies")
	}
	app.bearer = r.body["access_token"].(string)
	refresh := r.body["refresh_token"].(string)

	expect(t, app.do("GET", "/v1/me", nil), http.StatusOK)

	r2 := app.do("POST", "/v1/auth/token/refresh", map[string]any{"refresh_token": refresh})
	expect(t, r2, http.StatusOK)
	if r2.body["refresh_token"] == refresh || r2.body["access_token"] == nil {
		t.Errorf("mobile refresh body = %v", r2.body)
	}

	// Bearer requests don't need an Origin: logout without one works.
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/v1/auth/logout", nil)
	req.RemoteAddr = app.ip + ":1"
	req.Header.Set("Authorization", "Bearer "+r2.body["access_token"].(string))
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("bearer logout: status %d, body %s", rec.Code, rec.Body)
	}
	expect(t, app.do("POST", "/v1/auth/token/refresh", map[string]any{"refresh_token": r2.body["refresh_token"]}),
		http.StatusUnauthorized, "invalid_refresh_token")
	app.bearer = r2.body["access_token"].(string)
	expect(t, app.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestCookieWriteNeedsAllowedOrigin(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)

	req := httptest.NewRequestWithContext(t.Context(), "POST", "/v1/auth/logout", nil)
	req.RemoteAddr = b.ip + ":1"
	req.Header.Set("Origin", "https://evil.test")
	req.AddCookie(b.cookies["relay_access"])
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("cross-site logout: status %d, want 403", rec.Code)
	}
	expect(t, b.do("GET", "/v1/me", nil), http.StatusOK) // still signed in
}

func TestSignupDoesNotRevealExistingAccounts(t *testing.T) {
	e := newAuthEnv(t)
	b := e.browser(chromeMac)

	verified := e.verifiedUser()
	r := b.do("POST", "/v1/auth/signup", map[string]any{"email": verified, "password": testPassword, "first_name": "Eve"})
	expect(t, r, http.StatusAccepted)
	if subject, _ := e.lastEmail(verified); subject != "You already have a Relay account" {
		t.Errorf("verified owner got %q", subject)
	}

	unverified := newEmail()
	expect(t, b.do("POST", "/v1/auth/signup", map[string]any{"email": unverified, "password": testPassword, "first_name": "Ann"}), http.StatusAccepted)
	r2 := b.do("POST", "/v1/auth/signup", map[string]any{"email": unverified, "password": "0ther!Passw", "first_name": "Eve"})
	expect(t, r2, http.StatusAccepted)
	if r.raw != r2.raw {
		t.Errorf("responses differ: %s vs %s", r.raw, r2.raw)
	}
	subject, token := e.lastEmail(unverified)
	if subject != "Finish setting up your Relay account" || token == "" {
		t.Fatalf("unverified owner got %q", subject)
	}
	// The second signup's password was not applied.
	expect(t, b.do("POST", "/v1/auth/login", map[string]any{"email": unverified, "password": "0ther!Passw"}),
		http.StatusUnauthorized, "invalid_credentials")
	// The owner finishes by setting a password; that also verifies the email.
	expect(t, b.do("POST", "/v1/auth/password/reset", map[string]any{"token": token, "new_password": "Own3r!pass"}), http.StatusNoContent)
	expect(t, b.do("POST", "/v1/auth/login", map[string]any{"email": unverified, "password": "Own3r!pass"}), http.StatusOK)
}

func TestLoginFailures(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)

	unknown := b.do("POST", "/v1/auth/login", map[string]any{"email": newEmail(), "password": testPassword})
	wrong := b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "Wr0ng!pass"})
	expect(t, unknown, http.StatusUnauthorized, "invalid_credentials")
	expect(t, wrong, http.StatusUnauthorized, "invalid_credentials")
	if unknown.raw != wrong.raw {
		t.Errorf("unknown email and wrong password must look the same: %s vs %s", unknown.raw, wrong.raw)
	}

	for range 4 { // 5 failures in total for this email
		b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "Wr0ng!pass"})
	}
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusTooManyRequests, "too_many_attempts")
}

func TestPasswordRulesOnSignup(t *testing.T) {
	e := newAuthEnv(t)
	r := e.browser(chromeMac).do("POST", "/v1/auth/signup", map[string]any{"email": newEmail(), "password": "weak", "first_name": "Ada"})
	expect(t, r, http.StatusUnprocessableEntity, "weak_password")
	if d, _ := r.body["details"].([]any); len(d) != 4 {
		t.Errorf("want 4 unmet rules (length, upper, digit, symbol), got %v", r.body["details"])
	}
}

func TestPasswordResetSignsOutEverywhere(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	laptop := e.browser(chromeMac)
	laptop.login(email)

	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/password/forgot", map[string]any{"email": email}), http.StatusAccepted)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/password/forgot", map[string]any{"email": newEmail()}), http.StatusAccepted)
	subject, token := e.lastEmail(email)
	if subject != "Reset your Relay password" {
		t.Fatalf("subject = %q", subject)
	}

	expect(t, laptop.do("POST", "/v1/auth/password/reset", map[string]any{"token": token, "new_password": "N3w!password"}), http.StatusNoContent)
	expect(t, laptop.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
	expect(t, laptop.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
	expect(t, laptop.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}), http.StatusUnauthorized)
	expect(t, laptop.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "N3w!password"}), http.StatusOK)
}

func TestChangePasswordKeepsThisDevice(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	laptop, phone := e.browser(chromeMac), e.browser(safariPhone)
	laptop.login(email)
	phone.login(email)

	expect(t, laptop.do("POST", "/v1/auth/password/change", map[string]any{"current_password": "Wr0ng!pass", "new_password": "N3w!password"}),
		http.StatusUnprocessableEntity, "wrong_password")
	expect(t, laptop.do("POST", "/v1/auth/password/change", map[string]any{"current_password": testPassword, "new_password": "N3w!password"}),
		http.StatusNoContent)

	expect(t, laptop.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	expect(t, laptop.do("GET", "/v1/me", nil), http.StatusOK)
	expect(t, phone.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
	expect(t, phone.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestSessionsListAndRevoke(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	laptop, phone := e.browser(chromeMac), e.browser(safariPhone)
	phone.login(email)
	laptop.login(email)

	r := phone.do("GET", "/v1/auth/sessions", nil)
	expect(t, r, http.StatusOK)
	sessions := r.body["sessions"].([]any)
	if len(sessions) != 2 {
		t.Fatalf("sessions = %v", sessions)
	}
	first, second := sessions[0].(map[string]any), sessions[1].(map[string]any)
	if first["current"] != true || first["device_name"] != "Safari on iPhone" || first["device_type"] != "mobile" {
		t.Errorf("first session = %v", first)
	}
	if second["current"] != false || second["device_name"] != "Chrome on macOS" || second["last_ip"] != laptop.ip {
		t.Errorf("second session = %v", second)
	}

	expect(t, laptop.do("GET", "/v1/me", nil), http.StatusOK)
	expect(t, phone.do("DELETE", "/v1/auth/sessions/"+second["id"].(string), nil), http.StatusNoContent)
	// Revoking ends the laptop's access right away, not when its 15-minute token expires.
	expect(t, laptop.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
	expect(t, laptop.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized)
	expect(t, phone.do("GET", "/v1/me", nil), http.StatusOK)
	expect(t, phone.do("DELETE", "/v1/auth/sessions/"+uuid.NewString(), nil), http.StatusNotFound, "session_not_found")

	events := phone.do("GET", "/v1/auth/events", nil)
	expect(t, events, http.StatusOK)
	var names []string
	for _, ev := range events.body["events"].([]any) {
		names = append(names, ev.(map[string]any)["event"].(string))
	}
	for _, want := range []string{"signup", "email_verified", "login_succeeded", "session_revoked"} {
		if !strings.Contains(strings.Join(names, ","), want) {
			t.Errorf("events %v missing %s", names, want)
		}
	}
}

func TestLogoutAll(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	laptop, phone := e.browser(chromeMac), e.browser(safariPhone)
	laptop.login(email)
	phone.login(email)

	r := laptop.do("POST", "/v1/auth/logout-all", nil)
	expect(t, r, http.StatusNoContent)
	if laptop.cookies["relay_access"] != nil || laptop.cookies["relay_refresh"] != nil {
		t.Error("logout-all must clear this browser's cookies")
	}
	expect(t, phone.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized)
	expect(t, phone.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestTimesAreUTC(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)

	r := b.login(email)
	sessions := b.do("GET", "/v1/auth/sessions", nil)
	events := b.do("GET", "/v1/auth/events", nil)
	session := sessions.body["sessions"].([]any)[0].(map[string]any)
	event := events.body["events"].([]any)[0].(map[string]any)

	for name, v := range map[string]any{
		"access_expires_at":  r.body["access_expires_at"],
		"refresh_expires_at": r.body["refresh_expires_at"],
		"last_used_at":       session["last_used_at"],
		"session created_at": session["created_at"],
		"event created_at":   event["created_at"],
	} {
		if s, _ := v.(string); !strings.HasSuffix(s, "Z") {
			t.Errorf("%s = %v, want a UTC time ending in Z", name, v)
		}
	}
}

func TestUpdateProfileAndPreferences(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)

	r := b.do("PATCH", "/v1/me/profile", map[string]any{"display_name": "Countess", "last_name": ""})
	expect(t, r, http.StatusOK)
	p := r.body["profile"].(map[string]any)
	if p["display_name"] != "Countess" || p["last_name"] != nil || p["first_name"] != "Ada" {
		t.Errorf("profile = %v", p)
	}
	expect(t, b.do("PATCH", "/v1/me/profile", map[string]any{"first_name": "  "}), http.StatusUnprocessableEntity, "invalid_input")

	r = b.do("PATCH", "/v1/me/preferences", map[string]any{"locale": "en-GB", "timezone": "Europe/London"})
	expect(t, r, http.StatusOK)
	if pr := r.body["preferences"].(map[string]any); pr["locale"] != "en-GB" || pr["timezone"] != "Europe/London" {
		t.Errorf("preferences = %v", pr)
	}
	expect(t, b.do("PATCH", "/v1/me/preferences", map[string]any{"timezone": "Mars/Olympus"}), http.StatusUnprocessableEntity, "invalid_input")
}

func TestAuthRequired(t *testing.T) {
	e := newAuthEnv(t)
	b := e.browser(chromeMac)
	expect(t, b.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
	b.bearer = "not-a-jwt"
	expect(t, b.do("GET", "/v1/auth/sessions", nil), http.StatusUnauthorized, "unauthenticated")
}
