package handler_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/handler"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// The handler tests serve the billing routes through huma with the real
// permission and workspace middleware and a real *service.Service (the
// Handler holds a concrete service, so there is no fake to swap in). Signed-in
// users and their memberships are faked: userHeader names the user, and
// env.members says which workspaces they belong to and with what access.

const userHeader = "X-Test-User"

var errSignedOut = apperr.Unauthorized("unauthorized", "Sign in first")

type env struct {
	t       *testing.T
	api     humatest.TestAPI
	pool    *pgxpool.Pool
	members map[[2]uuid.UUID]authctx.Access
	staff   map[uuid.UUID]bool
}

// newEnv serves the billing routes. pool may be nil for tests that never
// reach a built service method.
func newEnv(t *testing.T, pool *pgxpool.Pool) *env {
	t.Helper()
	httpx.InstallErrorHandler()
	_, api := humatest.New(t, huma.DefaultConfig("Relay API", "test"))
	payments, err := payment.NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, api: api, pool: pool, members: map[[2]uuid.UUID]authctx.Access{}, staff: map[uuid.UUID]bool{}}
	svc := service.New(pool, payments, nil, service.Config{}, slog.Default())
	handler.Register(api, handler.NewHandler(svc), handler.Guards{
		Auth:   e.auth,
		Member: middleware.RequireWorkspace(api, e),
		Can: func(perms ...authctx.Permission) handler.Middleware {
			return middleware.RequirePermission(api, perms...)
		},
		Staff: middleware.RequireStaff(api, e),
	}, nil)
	return e
}

// dbEnv is newEnv with a migrated test database.
func dbEnv(t *testing.T) *env {
	t.Helper()
	return newEnv(t, dbtest.New(t))
}

// workspace inserts a workspace row, so service writes that reference it hold.
func (e *env) workspace() uuid.UUID {
	e.t.Helper()
	ws := uuid.New()
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO workspaces (id, name) VALUES ($1, 'Handler test')`, ws); err != nil {
		e.t.Fatal(err)
	}
	return ws
}

func (e *env) auth(ctx huma.Context, next func(huma.Context)) {
	id, err := uuid.Parse(ctx.Header(userHeader))
	if err != nil {
		_ = huma.WriteErr(e.api, ctx, http.StatusUnauthorized, "", errSignedOut)
		return
	}
	next(huma.WithContext(ctx, authctx.With(ctx.Context(), id, uuid.New())))
}

// ResolveAccess implements middleware.AccessResolver from e.members.
func (e *env) ResolveAccess(_ context.Context, workspaceID, userID uuid.UUID) (authctx.Access, bool, bool, error) {
	a, ok := e.members[[2]uuid.UUID{workspaceID, userID}]
	return a, false, ok, nil
}

// IsStaff implements middleware.StaffChecker from e.staff.
func (e *env) IsStaff(_ context.Context, userID uuid.UUID) (bool, error) { return e.staff[userID], nil }

// member is a user in a workspace with exactly perms (no owner bypass).
type member struct {
	e           *env
	userID      uuid.UUID
	workspaceID uuid.UUID
}

func (e *env) member(workspaceID uuid.UUID, perms ...authctx.Permission) member {
	var bits authctx.Bits
	for _, p := range perms {
		bits = bits.With(p)
	}
	m := member{e: e, userID: uuid.New(), workspaceID: workspaceID}
	e.members[[2]uuid.UUID{workspaceID, m.userID}] = authctx.Access{
		WorkspaceID: workspaceID, UserID: m.userID, Bits: bits, Scope: "all",
	}
	return m
}

// viewer can read billing; payer can also buy and change it.
func (e *env) viewer(ws uuid.UUID) member { return e.member(ws, authctx.PermBillingView) }

func (e *env) payer(ws uuid.UUID) member {
	return e.member(ws, authctx.PermBillingView, authctx.PermBillingPay)
}

type resp struct {
	code int
	body map[string]any
	raw  string
}

// do sends a request as m in m's workspace. Extra args are humatest's
// "Header: value" strings.
func (m member) do(method, path string, body any, args ...any) resp {
	args = append(args, userHeader+": "+m.userID.String(), middleware.WorkspaceHeader+": "+m.workspaceID.String())
	return m.e.do(method, path, body, args...)
}

func (e *env) do(method, path string, body any, args ...any) resp {
	e.t.Helper()
	if body != nil {
		args = append(args, body)
	}
	return decode(e.t, e.api.Do(method, path, args...))
}

func decode(t *testing.T, w *httptest.ResponseRecorder) resp {
	t.Helper()
	r := resp{code: w.Code, raw: w.Body.String()}
	if strings.TrimSpace(r.raw) != "" {
		if err := json.Unmarshal(w.Body.Bytes(), &r.body); err != nil {
			t.Fatalf("decode %q: %v", r.raw, err)
		}
	}
	return r
}

// expect checks the status and, if given, the error code.
func expect(t *testing.T, r resp, status int, code ...string) {
	t.Helper()
	if r.code != status {
		t.Fatalf("status %d, want %d; body: %s", r.code, status, r.raw)
	}
	if len(code) > 0 && r.body["code"] != code[0] {
		t.Fatalf("code %v, want %s; body: %s", r.body["code"], code[0], r.raw)
	}
}

// expectDenied checks a 403 that names the missing permission.
func expectDenied(t *testing.T, r resp, perm authctx.Permission) {
	t.Helper()
	expect(t, r, http.StatusForbidden, "permission_denied")
	details, _ := r.body["details"].([]any)
	if len(details) != 1 || details[0].(map[string]any)["message"] != perm.Key() {
		t.Fatalf("details %v, want the missing %s; body: %s", details, perm.Key(), r.raw)
	}
}

// expectReached checks that the request got past routing, guards and decoding
// to the service: whatever the service answers (here, usually that the price
// or subscription doesn't exist), it isn't a guard's or the decoder's refusal,
// a stub, or a crash. Success paths are covered by the flow tests.
func expectReached(t *testing.T, r resp) {
	t.Helper()
	switch code, _ := r.body["code"].(string); {
	case r.code >= http.StatusInternalServerError,
		code == "permission_denied", code == "workspace_not_found", code == "unauthorized",
		code == "validation_failed", code == "not_implemented":
		t.Fatalf("status %d, want an answer from the service; body: %s", r.code, r.raw)
	}
}

// expectKeys checks that m has exactly want as its keys.
func expectKeys(t *testing.T, m map[string]any, want ...string) {
	t.Helper()
	if len(m) != len(want) {
		t.Fatalf("keys %v, want %v", mapKeys(m), want)
	}
	for _, k := range want {
		if _, ok := m[k]; !ok {
			t.Fatalf("keys %v, want %v", mapKeys(m), want)
		}
	}
}

func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestListPlansIsPublic(t *testing.T) {
	e := dbEnv(t)
	// No user and no workspace: the catalog backs the public pricing page.
	expect(t, e.do(http.MethodGet, "/v1/billing/plans", nil), http.StatusOK)
}

func TestWorkspaceRoutesNeedAMember(t *testing.T) {
	e := newEnv(t, nil)
	ws := uuid.New()
	m := e.payer(ws)

	expect(t, e.do(http.MethodGet, "/v1/workspace/billing", nil,
		middleware.WorkspaceHeader+": "+ws.String()), http.StatusUnauthorized)
	stranger := member{e: e, userID: uuid.New(), workspaceID: ws}
	expect(t, stranger.do(http.MethodGet, "/v1/workspace/billing", nil), http.StatusNotFound, "workspace_not_found")
	other := member{e: e, userID: m.userID, workspaceID: uuid.New()}
	expect(t, other.do(http.MethodGet, "/v1/workspace/billing/invoices", nil), http.StatusNotFound, "workspace_not_found")
}

// route is one billing request with a body (and headers) that pass validation.
type route struct {
	method, path string
	body         any
	args         []any
}

func (r route) String() string { return r.method + " " + r.path }

// expectGuarded checks that a member without perm is refused before the
// service, and that a member with view and pay reaches it.
func expectGuarded(t *testing.T, routes []route, perm authctx.Permission) {
	t.Helper()
	e := dbEnv(t)
	ws := e.workspace()
	without := e.member(ws)
	if perm == authctx.PermBillingPay {
		without = e.viewer(ws)
	}
	payer := e.payer(ws)
	for _, r := range routes {
		t.Run(r.String(), func(t *testing.T) {
			expectDenied(t, without.do(r.method, r.path, r.body, r.args...), perm)
			expectReached(t, payer.do(r.method, r.path, r.body, r.args...))
		})
	}
}

func idempotencyKey() string { return "Idempotency-Key: " + uuid.NewString() }
