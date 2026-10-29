package app

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

var inviteTokenRe = regexp.MustCompile(`/invite#t=([A-Za-z0-9_-]+)`)

// inviteToken returns the link token from the newest invite email to addr.
func (e *authEnv) inviteToken(addr string) string {
	e.t.Helper()
	var args []byte
	err := e.a.db.QueryRow(context.Background(),
		`SELECT args FROM river_job WHERE kind = 'send_email' AND args->'message'->'to' ? $1 ORDER BY id DESC LIMIT 1`,
		addr).Scan(&args)
	if err != nil {
		e.t.Fatalf("no email queued for %s: %v", addr, err)
	}
	m := inviteTokenRe.FindSubmatch(args)
	if m == nil {
		e.t.Fatalf("not an invite email: %s", args)
	}
	return string(m[1])
}

// teamWorkspace is a workspace with its owner signed in.
type teamWorkspace struct {
	e     *authEnv
	owner *browser // sends X-Workspace-ID
	id    string
	base  string
	roles map[string]string // system key or name → role id
}

func newTeamWorkspace(e *authEnv) *teamWorkspace {
	e.t.Helper()
	owner := e.signedIn()
	created := owner.do("POST", "/v1/workspaces", map[string]any{"name": "Acme", "first_goal": "support", "start_mode": "setup"})
	expect(e.t, created, http.StatusCreated)
	id := created.body["id"].(string)
	w := &teamWorkspace{e: e, owner: owner.in(id), id: id, base: "/v1/workspace", roles: map[string]string{}}
	w.loadRoles()
	return w
}

func (w *teamWorkspace) loadRoles() {
	list := w.owner.do("GET", w.base+"/roles", nil)
	expect(w.e.t, list, http.StatusOK)
	for _, r := range list.body["roles"].([]any) {
		role := r.(map[string]any)
		key, _ := role["system_key"].(string)
		if key == "" {
			key = role["name"].(string)
		}
		w.roles[key] = role["id"].(string)
	}
}

// join invites a new verified user with the role and returns them signed in and
// scoped to the workspace, plus their user id.
func (w *teamWorkspace) join(role string) (*browser, string) {
	w.e.t.Helper()
	email := w.e.verifiedUser()
	expect(w.e.t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{email}, "role_id": w.roles[role],
	}), http.StatusCreated)
	b := w.e.browser(chromeMac).in(w.id)
	b.login(email)
	accepted := b.do("POST", "/v1/invitations/accept", map[string]any{"token": w.e.inviteToken(email)})
	expect(w.e.t, accepted, http.StatusOK)
	access := b.do("GET", w.base+"/me/access", nil)
	expect(w.e.t, access, http.StatusOK)
	var userID string
	members := w.owner.do("GET", w.base+"/members?q="+email, nil)
	expect(w.e.t, members, http.StatusOK)
	for _, m := range members.body["members"].([]any) {
		if m.(map[string]any)["email"] == email {
			userID = m.(map[string]any)["user_id"].(string)
		}
	}
	if userID == "" {
		w.e.t.Fatalf("%s not in members: %s", email, members.raw)
	}
	return b, userID
}

func permissionsOf(t *testing.T, r resp) []string {
	t.Helper()
	var out []string
	for _, p := range r.body["permissions"].([]any) {
		out = append(out, p.(string))
	}
	return out
}

func TestPermissionCatalogMatchesDatabase(t *testing.T) {
	e := newAuthEnv(t)
	b := e.signedIn()
	r := b.do("GET", "/v1/permissions", nil)
	expect(t, r, http.StatusOK)
	var keys []string
	for _, p := range r.body["permissions"].([]any) {
		keys = append(keys, p.(map[string]any)["key"].(string))
	}
	// Bits must match too: the access query shifts by the database's bit.
	rows, err := e.a.db.Query(t.Context(), `SELECT key, bit FROM permissions ORDER BY bit`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var key string
		var bit int16
		if err := rows.Scan(&key, &bit); err != nil {
			t.Fatal(err)
		}
		p, ok := authctx.PermissionByKey(key)
		if !ok || int16(p) != bit {
			t.Fatalf("permission %s bit %d: authctx has %v (%v)", key, bit, p, ok)
		}
		n++
	}
	if n != len(authctx.AllPermissions()) || len(keys) != n {
		t.Fatalf("database has %d permissions, authctx %d, API %d", n, len(authctx.AllPermissions()), len(keys))
	}
}

func TestOwnerHasEverything(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	for _, k := range []string{"owner", "admin", "agent", "viewer"} {
		if w.roles[k] == "" {
			t.Fatalf("missing system role %s: %v", k, w.roles)
		}
	}
	r := w.owner.do("GET", w.base+"/me/access", nil)
	expect(t, r, http.StatusOK)
	if r.body["is_owner"] != true || len(permissionsOf(t, r)) != len(authctx.AllPermissions()) {
		t.Fatalf("owner access = %s", r.raw)
	}
	if r.header.Get("X-Access-Version") == "" {
		t.Fatal("no X-Access-Version header")
	}

	// A stranger can't tell the workspace exists.
	expect(t, e.signedIn().in(w.id).do("GET", w.base+"/members", nil), http.StatusNotFound, "workspace_not_found")
}

func TestInviteAcceptAndRoleChecks(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)

	email := e.verifiedUser()
	expect(t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{email}, "role_id": w.roles["owner"],
	}), http.StatusForbidden, "owner_role")
	expect(t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{email}, "role_id": w.roles["agent"], "personal_message": "Welcome aboard",
	}), http.StatusCreated)
	expect(t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{email}, "role_id": w.roles["agent"],
	}), http.StatusConflict, "invite_pending")
	token := e.inviteToken(email)

	// The link shows what's on offer before sign-in.
	preview := e.browser(chromeMac).do("POST", "/v1/invitations/preview", map[string]any{"token": token})
	expect(t, preview, http.StatusOK)
	if preview.body["role_name"] != "Agent" || preview.body["workspace_name"] != "Acme" || preview.body["state"] != "valid" {
		t.Fatalf("preview = %s", preview.raw)
	}

	// Only the invited address can accept.
	other := e.signedIn()
	expect(t, other.do("POST", "/v1/invitations/accept", map[string]any{"token": token}),
		http.StatusForbidden, "invite_email_mismatch")

	agent := e.browser(chromeMac).in(w.id)
	agent.login(email)
	expect(t, agent.do("POST", "/v1/invitations/accept", map[string]any{"token": token}), http.StatusOK)
	expect(t, agent.do("POST", "/v1/invitations/accept", map[string]any{"token": token}), http.StatusConflict)

	access := agent.do("GET", w.base+"/me/access", nil)
	expect(t, access, http.StatusOK)
	perms := permissionsOf(t, access)
	if access.body["is_owner"] != false || !slices.Contains(perms, "inbox.reply") || slices.Contains(perms, "team.roles") {
		t.Fatalf("agent access = %s", access.raw)
	}
	if access.body["chat_scope"] != "team" {
		t.Fatalf("agent scope = %v", access.body["chat_scope"])
	}

	// Agents can see the team but not change it.
	expect(t, agent.do("GET", w.base+"/members", nil), http.StatusOK)
	expect(t, agent.do("GET", w.base+"/invitations", nil), http.StatusForbidden, "permission_denied")
	expect(t, agent.do("POST", w.base+"/roles", map[string]any{"name": "Mine"}), http.StatusForbidden, "permission_denied")
	// Setup steps are the owner's; numbers need integrations.manage.
	expect(t, agent.do("POST", w.base+"/onboarding/skip-whatsapp", nil), http.StatusForbidden, "owner_only")
	expect(t, agent.do("POST", w.base+"/whatsapp/connections", map[string]any{}), http.StatusForbidden, "permission_denied")

	seats := w.owner.do("GET", w.base+"/seats", nil)
	expect(t, seats, http.StatusOK)
	if seats.body["used"] != float64(2) {
		t.Fatalf("seats = %s", seats.raw)
	}
}

func TestOverridesCustomRolesAndEscalation(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	agent, agentID := w.join("agent")
	member := w.base + "/members/" + agentID

	// A grant lets the agent invite; a deny takes replying away.
	expect(t, w.owner.do("POST", member+"/overrides", map[string]any{
		"permission_key": "team.invite", "effect": "grant", "reason": "Hiring week",
	}), http.StatusNoContent)
	expect(t, w.owner.do("POST", member+"/overrides", map[string]any{
		"permission_key": "inbox.reply", "effect": "deny",
	}), http.StatusNoContent)
	expect(t, agent.do("GET", w.base+"/invitations", nil), http.StatusOK)
	perms := permissionsOf(t, agent.do("GET", w.base+"/me/access", nil))
	if slices.Contains(perms, "inbox.reply") || !slices.Contains(perms, "team.invite") {
		t.Fatalf("after overrides: %v", perms)
	}

	// The agent can't hand out more than they have.
	expect(t, w.owner.do("POST", member+"/overrides", map[string]any{
		"permission_key": "team.roles", "effect": "grant",
	}), http.StatusNoContent)
	expect(t, agent.do("POST", w.base+"/roles", map[string]any{
		"name": "Payer", "color": "red", "chat_scope": "all", "permissions": []string{"billing.pay"},
	}), http.StatusForbidden, "cannot_grant")
	expect(t, agent.do("PUT", member+"/role", map[string]any{"role_id": w.roles["admin"]}),
		http.StatusForbidden, "cannot_change_self")

	// A custom role (no system key) resolves like any other.
	created := w.owner.do("POST", w.base+"/roles", map[string]any{
		"name": "Support lead", "color": "blue", "chat_scope": "all",
		"permissions": []string{"inbox.reply", "contacts.view", "contacts.export"},
	})
	expect(t, created, http.StatusCreated)
	expect(t, w.owner.do("PUT", member+"/role", map[string]any{"role_id": created.body["id"]}), http.StatusNoContent)
	access := agent.do("GET", w.base+"/me/access", nil)
	perms = permissionsOf(t, access)
	if !slices.Contains(perms, "contacts.export") || slices.Contains(perms, "inbox.reply") || access.body["chat_scope"] != "all" {
		t.Fatalf("custom role access = %s", access.raw)
	}

	// Editing the role reaches its members on their next request.
	expect(t, w.owner.do("PATCH", w.base+"/roles/"+created.body["id"].(string), map[string]any{
		"permissions": []string{"inbox.reply"}, "expected_version": created.body["version"],
	}), http.StatusOK)
	perms = permissionsOf(t, agent.do("GET", w.base+"/me/access", nil))
	if slices.Contains(perms, "contacts.export") {
		t.Fatalf("role edit not applied: %v", perms)
	}
	expect(t, w.owner.do("PATCH", w.base+"/roles/"+created.body["id"].(string), map[string]any{
		"permissions": []string{"inbox.reply"}, "expected_version": created.body["version"],
	}), http.StatusConflict, "role_changed")

	// System roles can be edited but not deleted.
	expect(t, w.owner.do("DELETE", w.base+"/roles/"+w.roles["agent"]+"?move_to_role_id="+w.roles["viewer"], nil),
		http.StatusConflict, "system_role")
	// Deleting a custom role moves its members first.
	expect(t, w.owner.do("DELETE", w.base+"/roles/"+created.body["id"].(string)+"?move_to_role_id="+w.roles["viewer"], nil),
		http.StatusNoContent)
	if perms = permissionsOf(t, agent.do("GET", w.base+"/me/access", nil)); slices.Contains(perms, "contacts.edit") {
		t.Fatalf("not moved to viewer: %v", perms)
	}

	log := w.owner.do("GET", w.base+"/access-log", nil)
	expect(t, log, http.StatusOK)
	var actions []string
	for _, entry := range log.body["entries"].([]any) {
		actions = append(actions, entry.(map[string]any)["action"].(string))
	}
	for _, want := range []string{"invite_accepted", "permission_granted", "permission_denied", "role_created", "role_changed", "role_updated"} {
		if !slices.Contains(actions, want) {
			t.Fatalf("access log has no %s: %v", want, actions)
		}
	}
}

func TestSuspendRestoreRemove(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	agent, agentID := w.join("agent")
	member := w.base + "/members/" + agentID

	expect(t, w.owner.do("POST", member+"/suspend", map[string]any{"reason": "On leave"}), http.StatusNoContent)
	expect(t, agent.do("GET", w.base+"/me/access", nil), http.StatusForbidden, "member_suspended")
	expect(t, w.owner.do("POST", member+"/restore", nil), http.StatusNoContent)
	expect(t, agent.do("GET", w.base+"/me/access", nil), http.StatusOK)

	expect(t, w.owner.do("POST", member+"/remove", map[string]any{"confirm_name": "Someone else"}),
		http.StatusUnprocessableEntity, "confirm_name_mismatch")
	expect(t, w.owner.do("POST", member+"/remove", map[string]any{"confirm_name": "Ada Lovelace"}), http.StatusNoContent)
	expect(t, agent.do("GET", w.base+"/me/access", nil), http.StatusNotFound, "workspace_not_found")

	// The owner can't leave or be removed.
	expect(t, w.owner.do("POST", w.base+"/me/leave", nil), http.StatusConflict, "owner_must_transfer")
}

func TestTeamsScopeMembers(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	agent, agentID := w.join("agent")

	team := w.owner.do("POST", w.base+"/teams", map[string]any{"name": "Billing", "color": "amber"})
	expect(t, team, http.StatusCreated)
	teamID := team.body["id"].(string)
	expect(t, w.owner.do("POST", w.base+"/teams", map[string]any{"name": "billing"}), http.StatusConflict, "team_name_taken")
	expect(t, w.owner.do("PUT", w.base+"/teams/"+teamID+"/members", map[string]any{"user_ids": []string{agentID}}), http.StatusOK)

	access := agent.do("GET", w.base+"/me/access", nil)
	var teams []string
	raw, _ := json.Marshal(access.body["team_ids"])
	_ = json.Unmarshal(raw, &teams)
	if !slices.Contains(teams, teamID) {
		t.Fatalf("agent teams = %s", access.raw)
	}
	expect(t, w.owner.do("DELETE", w.base+"/teams/"+teamID, nil), http.StatusNoContent)
	access = agent.do("GET", w.base+"/me/access", nil)
	if ids := access.body["team_ids"].([]any); len(ids) != 0 {
		t.Fatalf("team still listed after delete: %s", access.raw)
	}
}

func TestSignupFromInvite(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	email := newEmail()
	expect(t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{email}, "role_id": w.roles["agent"],
	}), http.StatusCreated)
	token := e.inviteToken(email)

	b := e.browser(chromeMac).in(w.id)
	expect(t, b.do("POST", "/v1/auth/signup/invite", map[string]any{
		"token": token, "password": testPassword, "first_name": "Karim", "accept_terms": false,
	}), http.StatusUnprocessableEntity, "terms_required")
	joined := b.do("POST", "/v1/auth/signup/invite", map[string]any{
		"token": token, "password": testPassword, "first_name": "Karim", "last_name": "Hossain",
		"timezone": "Asia/Dhaka", "accept_terms": true,
	})
	expect(t, joined, http.StatusOK)
	if joined.body["workspace_id"] == nil || joined.body["join_error"] != nil {
		t.Fatalf("invite signup = %s", joined.raw)
	}
	// Signed in, verified and a member: no separate verification step.
	access := b.do("GET", w.base+"/me/access", nil)
	expect(t, access, http.StatusOK)
	if !slices.Contains(permissionsOf(t, access), "inbox.reply") {
		t.Fatalf("access = %s", access.raw)
	}
	// The link is used up, and an existing account must log in instead.
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/signup/invite", map[string]any{
		"token": token, "password": testPassword, "first_name": "Karim", "accept_terms": true,
	}), http.StatusConflict, "invite_accepted")
	other := e.verifiedUser()
	expect(t, w.owner.do("POST", w.base+"/invitations", map[string]any{
		"emails": []string{other}, "role_id": w.roles["viewer"],
	}), http.StatusCreated)
	expect(t, e.browser(chromeMac).do("POST", "/v1/auth/signup/invite", map[string]any{
		"token": e.inviteToken(other), "password": testPassword, "first_name": "Ada", "accept_terms": true,
	}), http.StatusConflict, "account_exists")
}
