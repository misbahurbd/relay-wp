package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	authjobs "github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/jobs"
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
	t     *testing.T
	a     *App
	h     http.Handler
	fake  *fakeProvider
	graph *fakeGraph
}

func newAuthEnv(t *testing.T) *authEnv {
	t.Helper()
	pool := dbtest.New(t)
	// The plan new workspaces trial (TRIAL_PLAN): 14 days, no card, no caps.
	if _, err := pool.Exec(t.Context(), `INSERT INTO plans (code, name, trial_days, limits)
		VALUES ('growth', 'Growth', 14, '{"members": -1, "whatsapp_numbers": -1, "contacts": -1}')
		ON CONFLICT (code, version) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	seed, _, err := jwt.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	q, err := queue.New(pool, nil, 0, 0, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	fake := newFakeProvider(t)
	graph := newFakeGraph(t)
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
				AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 720 * time.Hour, SessionMaxAge: 2160 * time.Hour, CookieSecure: true,
				APIBaseURL: testAPI, AllowedRedirects: []string{testAppRedirect}, ReauthMaxAge: 10 * time.Minute,
				EmailCodeKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)), TermsVersion: "test",
			},
			Trial:    config.Trial{Plan: "growth"},
			WhatsApp: testWhatsAppConfig(),
		},
		log: slog.Default(), db: pool, queue: q, oauth: fake.config(), graphURL: graph.srv.URL,
	}
	if err := a.buildFeatures(); err != nil {
		t.Fatal(err)
	}
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &authEnv{t: t, a: a, h: h, fake: fake, graph: graph}
}

// browser is one client: it keeps cookies like a browser, or a bearer token like the app.
type browser struct {
	env     *authEnv
	ip      string
	ua      string
	cookies map[string]*http.Cookie
	bearer  string
	device  string // X-Device-Name, as the mobile app sends it
	// workspace is sent as X-Workspace-ID when set.
	workspace string
	// idempotency is sent as Idempotency-Key when set.
	idempotency string
}

func (e *authEnv) browser(ua string) *browser {
	return &browser{
		env: e, ua: ua, cookies: map[string]*http.Cookie{},
		ip: fmt.Sprintf("10.%d.%d.%d", rand.IntN(250), rand.IntN(250), rand.IntN(250)+1), //nolint:gosec // test data
	}
}

// in returns b scoped to workspace: same session, with X-Workspace-ID set.
func (b *browser) in(workspace string) *browser {
	c := *b
	c.workspace = workspace
	return &c
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
	if b.workspace != "" {
		req.Header.Set("X-Workspace-ID", b.workspace)
	}
	if b.idempotency != "" {
		req.Header.Set("Idempotency-Key", b.idempotency)
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

var (
	tokenRe       = regexp.MustCompile(`token=([A-Za-z0-9_-]+)`)
	codeSubjectRe = regexp.MustCompile(`^([0-9]{6}) is your Relay (?:verification|confirmation) code$`)
)

// lastCode returns the 6-digit code from the newest verification email to addr.
func (e *authEnv) lastCode(addr string) string {
	e.t.Helper()
	subject, _ := e.lastEmail(addr)
	m := codeSubjectRe.FindStringSubmatch(subject)
	if m == nil {
		e.t.Fatalf("not a verification code email: %q", subject)
	}
	return m[1]
}

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
		"email": email, "password": testPassword, "first_name": "Ada", "last_name": "Lovelace", "timezone": "Asia/Dhaka", "accept_terms": true,
	}), http.StatusAccepted)
	_, token := e.lastEmail(email)
	expect(e.t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token, "password": testPassword}), http.StatusNoContent)
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
		"email": email, "password": testPassword, "first_name": "Ada", "last_name": "Lovelace", "timezone": "Asia/Dhaka", "accept_terms": true,
	}), http.StatusAccepted)

	// Correct password but unverified: told to verify.
	expect(t, b.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusForbidden, "email_not_verified")

	subject, token := e.lastEmail(email)
	if !codeSubjectRe.MatchString(subject) || token == "" {
		t.Fatalf("verification email: subject=%q token=%q", subject, token)
	}
	// The link alone isn't enough for a password account; a refusal leaves it usable.
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token}),
		http.StatusUnprocessableEntity, "password_required")
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token, "password": "Wrong#Pass1x"}),
		http.StatusUnprocessableEntity, "wrong_password")
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token, "password": testPassword}), http.StatusNoContent)
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": token, "password": testPassword}),
		http.StatusUnprocessableEntity, "invalid_link")

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

func TestVerifyLinkCantActivateSomeoneElsesPassword(t *testing.T) {
	e := newAuthEnv(t)
	victim := newEmail()

	// An attacker signs up with the victim's address, choosing the password.
	attacker := e.browser(chromeMac)
	expect(t, attacker.do("POST", "/v1/auth/signup", map[string]any{
		"email": victim, "password": testPassword, "first_name": "Mallory", "accept_terms": true,
	}), http.StatusAccepted)

	// The victim opens the link (or a mail scanner does). Without the attacker's
	// password the account stays unverified, so that password stays useless.
	_, token := e.lastEmail(victim)
	owner := e.browser(chromeMac)
	expect(t, owner.do("POST", "/v1/auth/verify-email", map[string]any{"token": token}),
		http.StatusUnprocessableEntity, "password_required")
	expect(t, owner.do("POST", "/v1/auth/verify-email", map[string]any{"token": token, "password": "Guess#Pass1x"}),
		http.StatusUnprocessableEntity, "wrong_password")
	expect(t, attacker.do("POST", "/v1/auth/login", map[string]any{"email": victim, "password": testPassword}),
		http.StatusForbidden, "email_not_verified")

	// The victim sets their own password instead: the attacker's is gone.
	expect(t, owner.do("POST", "/v1/auth/password/forgot", map[string]any{"email": victim}), http.StatusAccepted)
	_, reset := e.lastEmail(victim)
	expect(t, owner.do("POST", "/v1/auth/password/reset", map[string]any{"token": reset, "new_password": "Own#Passw0rd!x"}),
		http.StatusNoContent)
	expect(t, attacker.do("POST", "/v1/auth/login", map[string]any{"email": victim, "password": testPassword}),
		http.StatusUnauthorized, "invalid_credentials")
}

func TestVerifyLinkWithoutPasswordForSocialAccount(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	// Older versions created social accounts with an unproven email and no password.
	e.exec(`INSERT INTO users (id, email) VALUES ($1, $2)`, uuid.New(), email)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/verify-email/resend", map[string]any{"email": email}), http.StatusAccepted)

	// Nobody chose a password, so the link alone verifies.
	_, token := e.lastEmail(email)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/verify-email", map[string]any{"token": token}), http.StatusNoContent)
	var verified bool
	if err := e.a.db.QueryRow(t.Context(), `SELECT email_verified_at IS NOT NULL FROM users WHERE email = $1`, email).Scan(&verified); err != nil || !verified {
		t.Errorf("verified = %v (%v)", verified, err)
	}
}

func TestUnverifiedAccountHasNoSession(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)
	// However an account ends up unverified, its sessions stop working at once.
	e.exec(`UPDATE users SET email_verified_at = NULL WHERE email = $1`, email)
	expect(t, b.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
}

func TestCleanupDeletesStaleAuthRows(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)
	_ = e.browser(chromeMac).startOAuth("/v1/auth/oauth/google/start")

	const user = `(SELECT id FROM users WHERE email = $1)`
	e.exec(`UPDATE auth_sessions SET revoked_at = now() - interval '31 days' WHERE user_id = `+user, email)
	e.exec(`UPDATE email_tokens SET expires_at = now() - interval '1 minute' WHERE user_id = `+user, email)
	e.exec(`UPDATE oauth_flows SET expires_at = now() - interval '1 minute'`)
	e.exec(`UPDATE auth_events SET created_at = now() - interval '181 days' WHERE user_id = `+user, email)

	if err := authjobs.NewCleanupWorker(e.a.db).Work(t.Context(), &river.Job[authjobs.CleanupArgs]{}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`SELECT count(*) FROM auth_sessions WHERE user_id = ` + user,
		`SELECT count(*) FROM refresh_tokens WHERE session_id IN (SELECT id FROM auth_sessions WHERE user_id = ` + user + `)`,
		`SELECT count(*) FROM email_tokens WHERE user_id = ` + user,
		`SELECT count(*) FROM auth_events WHERE user_id = ` + user,
	} {
		var n int
		if err := e.a.db.QueryRow(t.Context(), q, email).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s = %d (%v), want 0", q, n, err)
		}
	}
	var flows int
	if err := e.a.db.QueryRow(t.Context(), `SELECT count(*) FROM oauth_flows WHERE expires_at < now()`).Scan(&flows); err != nil || flows != 0 {
		t.Errorf("expired oauth flows = %d (%v)", flows, err)
	}
}

func TestSignupNeedsTerms(t *testing.T) {
	e := newAuthEnv(t)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/signup", map[string]any{
		"email": newEmail(), "password": testPassword, "first_name": "Ada", "accept_terms": false,
	}), http.StatusUnprocessableEntity, "terms_required")
}

func TestVerifyEmailCode(t *testing.T) {
	e := newAuthEnv(t)
	b := e.browser(chromeMac)
	email := newEmail()
	expect(t, b.do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Ada", "accept_terms": true,
	}), http.StatusAccepted)
	code := e.lastCode(email)
	_, link := e.lastEmail(email)
	wrong := fmt.Sprintf("%06d", (mustAtoi(t, code)+1)%1_000_000)

	// Wrong code, and an email with no account, get the same answer, whatever the password.
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": wrong, "password": "Wrong#Pass1x"}),
		http.StatusUnprocessableEntity, "invalid_code")
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": newEmail(), "code": code}),
		http.StatusUnprocessableEntity, "invalid_code")

	// The right code still needs the password chosen at sign-up; a refusal keeps the code usable.
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code}),
		http.StatusUnprocessableEntity, "password_required")
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code, "password": "Wrong#Pass1x"}),
		http.StatusUnprocessableEntity, "wrong_password")

	// Both right: verifies and signs in, like login.
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code, "password": testPassword}), http.StatusOK)
	if b.cookies["relay_access"] == nil || b.cookies["relay_refresh"] == nil {
		t.Fatal("code verification must set auth cookies")
	}
	me := b.do("GET", "/v1/me", nil)
	expect(t, me, http.StatusOK)
	if me.body["user"].(map[string]any)["email_verified"] != true {
		t.Errorf("user = %v", me.body["user"])
	}

	// The code and the link from the same email are both spent.
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code, "password": testPassword}),
		http.StatusUnprocessableEntity, "invalid_code")
	expect(t, b.do("POST", "/v1/auth/verify-email", map[string]any{"token": link, "password": testPassword}),
		http.StatusUnprocessableEntity, "invalid_link")

	var termsVersion *string
	if err := e.a.db.QueryRow(t.Context(), `SELECT terms_version FROM users WHERE email = $1`, email).Scan(&termsVersion); err != nil {
		t.Fatal(err)
	}
	if termsVersion == nil || *termsVersion != "test" {
		t.Errorf("terms_version = %v", termsVersion)
	}
}

func TestVerifyEmailCodeAttemptsAndResend(t *testing.T) {
	e := newAuthEnv(t)
	b := e.browser(chromeMac)
	email := newEmail()
	expect(t, b.do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Ada", "accept_terms": true,
	}), http.StatusAccepted)
	code := e.lastCode(email)
	wrong := fmt.Sprintf("%06d", (mustAtoi(t, code)+1)%1_000_000)

	for range 4 {
		expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": wrong}),
			http.StatusUnprocessableEntity, "invalid_code")
	}
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": wrong}),
		http.StatusTooManyRequests, "too_many_attempts")
	// Dead now, even with the right code.
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code, "password": testPassword}),
		http.StatusTooManyRequests, "too_many_attempts")

	// A new code replaces it.
	expect(t, b.do("POST", "/v1/auth/verify-email/resend", map[string]any{"email": email}), http.StatusAccepted)
	fresh := e.lastCode(email)
	if fresh != code {
		expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": code}),
			http.StatusUnprocessableEntity, "invalid_code")
	}
	expect(t, b.do("POST", "/v1/auth/verify-email/code", map[string]any{"email": email, "code": fresh, "password": testPassword}), http.StatusOK)
}

func TestVerifyCodesAreCappedPerAccount(t *testing.T) {
	e := newAuthEnv(t)
	email := newEmail()
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/signup", map[string]any{
		"email": email, "password": testPassword, "first_name": "Ada", "accept_terms": true,
	}), http.StatusAccepted)

	// Each resend comes from a new IP, so only the per-account cap can stop it:
	// sign-up's code plus 3 more an hour, each with 5 tries.
	for range 6 {
		expect(t, e.browser(chromeMac).do("POST", "/v1/auth/verify-email/resend", map[string]any{"email": email}), http.StatusAccepted)
	}
	var codes int
	if err := e.a.db.QueryRow(t.Context(),
		`SELECT count(*) FROM email_codes c JOIN users u ON u.id = c.user_id WHERE u.email = $1`, email).Scan(&codes); err != nil {
		t.Fatal(err)
	}
	if codes != 4 {
		t.Errorf("codes sent in an hour = %d, want 4", codes)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
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

	// Replaying the first (already rotated) token after the grace window = theft: the whole session ends.
	e.exec(`UPDATE refresh_tokens SET used_at = now() - interval '1 minute' WHERE used_at IS NOT NULL`)
	attacker := e.browser(chromeMac)
	attacker.cookies["relay_refresh"] = &http.Cookie{Name: "relay_refresh", Value: first, Path: "/v1/auth/token/refresh"} //nolint:gosec // test client
	expect(t, attacker.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")

	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
	// The session is gone, so its still-unexpired access token stops working too.
	expect(t, b.do("GET", "/v1/me", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestConcurrentTabsRefreshWithinGrace(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	tab1 := e.browser(chromeMac)
	tab1.login(email)

	// A second tab shares the cookies and refreshes right after the first one rotated them.
	tab2 := e.browser(chromeMac)
	for name, c := range tab1.cookies {
		tab2.cookies[name] = c
	}
	expect(t, tab1.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	expect(t, tab2.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	if tab1.cookies["relay_refresh"].Value == tab2.cookies["relay_refresh"].Value {
		t.Fatal("each refresh must get its own token")
	}

	// Neither looked like theft: both tabs stay signed in and can keep refreshing.
	for _, tab := range []*browser{tab1, tab2} {
		expect(t, tab.do("GET", "/v1/me", nil), http.StatusOK)
		expect(t, tab.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	}
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
	r := b.do("POST", "/v1/auth/signup", map[string]any{"email": verified, "password": testPassword, "first_name": "Eve", "accept_terms": true})
	expect(t, r, http.StatusAccepted)
	if subject, _ := e.lastEmail(verified); subject != "You already have a Relay account" {
		t.Errorf("verified owner got %q", subject)
	}

	unverified := newEmail()
	expect(t, b.do("POST", "/v1/auth/signup", map[string]any{"email": unverified, "password": testPassword, "first_name": "Ann", "accept_terms": true}), http.StatusAccepted)
	r2 := b.do("POST", "/v1/auth/signup", map[string]any{"email": unverified, "password": "0ther!Passw", "first_name": "Eve", "accept_terms": true})
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
	attacker := e.browser(chromeMac)

	unknown := attacker.do("POST", "/v1/auth/login", map[string]any{"email": newEmail(), "password": testPassword})
	wrong := attacker.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "Wr0ng!pass"})
	expect(t, unknown, http.StatusUnauthorized, "invalid_credentials")
	expect(t, wrong, http.StatusUnauthorized, "invalid_credentials")
	if unknown.raw != wrong.raw {
		t.Errorf("unknown email and wrong password must look the same: %s vs %s", unknown.raw, wrong.raw)
	}

	for range 4 { // 5 failures from this IP
		attacker.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "Wr0ng!pass"})
	}
	expect(t, attacker.do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusTooManyRequests, "too_many_attempts")
	// The owner, elsewhere, is not locked out by a stranger's guesses.
	e.browser(chromeMac).login(email)

	// Unknown emails lock the same way, so a 429 says nothing about who has an account.
	ghost, prober := newEmail(), e.browser(chromeMac)
	for range 5 {
		prober.do("POST", "/v1/auth/login", map[string]any{"email": ghost, "password": "Wr0ng!pass"})
	}
	expect(t, prober.do("POST", "/v1/auth/login", map[string]any{"email": ghost, "password": "Wr0ng!pass"}),
		http.StatusTooManyRequests, "too_many_attempts")
}

func TestLoginLockoutAcrossManyIPs(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	// 50 wrong passwords spread over many IPs lock the account everywhere for a while.
	for range 50 {
		e.browser(chromeMac).do("POST", "/v1/auth/login", map[string]any{"email": email, "password": "Wr0ng!pass"})
	}
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/login", map[string]any{"email": email, "password": testPassword}),
		http.StatusTooManyRequests, "too_many_attempts")
}

func TestChangePasswordGuessesAreLimited(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	thief := e.browser(chromeMac)
	thief.login(email)
	for range 5 {
		expect(t, thief.do("POST", "/v1/auth/password/change", map[string]any{"current_password": "Wr0ng!pass", "new_password": "New#Passw0rd!x"}),
			http.StatusUnprocessableEntity, "wrong_password")
	}
	expect(t, thief.do("POST", "/v1/auth/password/change", map[string]any{"current_password": testPassword, "new_password": "New#Passw0rd!x"}),
		http.StatusTooManyRequests, "too_many_attempts")
}

func TestSessionMaxAge(t *testing.T) {
	e := newAuthEnv(t)
	email := e.verifiedUser()
	b := e.browser(chromeMac)
	b.login(email)

	// An hour from the 90-day cap, a refresh gets an hour, not another 30 days.
	const mine = `user_id = (SELECT id FROM users WHERE email = $1)`
	e.exec(`UPDATE auth_sessions SET created_at = now() - interval '2159 hours' WHERE `+mine, email)
	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusOK)
	var exp time.Time
	if err := e.a.db.QueryRow(t.Context(), `SELECT expires_at FROM auth_sessions WHERE `+mine, email).Scan(&exp); err != nil {
		t.Fatal(err)
	}
	if exp.After(time.Now().Add(2 * time.Hour)) {
		t.Errorf("session expires %s, want within the 90-day cap", exp)
	}

	// Past the cap, refreshing ends.
	e.exec(`UPDATE auth_sessions SET created_at = now() - interval '91 days' WHERE `+mine, email)
	expect(t, b.do("POST", "/v1/auth/token/refresh", nil), http.StatusUnauthorized, "invalid_refresh_token")
}

func TestPasswordRulesOnSignup(t *testing.T) {
	e := newAuthEnv(t)
	r := e.browser(chromeMac).do("POST", "/v1/auth/signup", map[string]any{"email": newEmail(), "password": "weak", "first_name": "Ada", "accept_terms": true})
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
