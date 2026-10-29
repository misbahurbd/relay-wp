package handler

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Middleware is a huma middleware (middleware.RequireAuth, middleware.RequireWorkspace, …).
type Middleware = func(huma.Context, func(huma.Context))

// Guards are the middlewares the routes need, built in internal/app.
type Guards struct {
	Auth   Middleware
	Member Middleware
	// Can returns a middleware that requires every one of perms.
	Can func(perms ...authctx.Permission) Middleware
}

// Register adds the team routes.
func Register(api huma.API, h *Handler, g Guards, security []map[string][]string) {
	ws := func(id, method, path, summary string, status int, mw ...Middleware) huma.Operation {
		return huma.Operation{
			OperationID: id, Method: method, Path: "/v1/workspaces/{workspaceId}" + path, Summary: summary,
			Tags: []string{"team"}, Security: security, DefaultStatus: status,
			Middlewares: append(huma.Middlewares{g.Auth, g.Member}, mw...),
		}
	}
	roles, invite, remove := g.Can(authctx.PermTeamRoles), g.Can(authctx.PermTeamInvite), g.Can(authctx.PermTeamRemove)
	ok, created, none := http.StatusOK, http.StatusCreated, http.StatusNoContent

	huma.Register(api, ws("get-my-access", http.MethodGet, "/me/access",
		"What I can do in this workspace", ok), h.myAccess)
	huma.Register(api, ws("set-availability", http.MethodPatch, "/me/availability",
		"Set my status to online, away or offline", none), h.setAvailability)
	huma.Register(api, ws("leave-workspace", http.MethodPost, "/me/leave",
		"Leave this workspace", none), h.leave)
	huma.Register(api, ws("get-seats", http.MethodGet, "/seats",
		"Seats on the plan and how they're used", ok), h.seats)
	huma.Register(api, huma.Operation{
		OperationID: "list-permissions", Method: http.MethodGet, Path: "/v1/permissions",
		Summary: "The permission catalog", Tags: []string{"team"}, Security: security,
		Middlewares: huma.Middlewares{g.Auth},
	}, h.permissions)

	huma.Register(api, ws("list-roles", http.MethodGet, "/roles", "Roles with their permissions", ok), h.listRoles)
	huma.Register(api, ws("create-role", http.MethodPost, "/roles", "Create a custom role", created, roles), h.createRole)
	huma.Register(api, ws("preview-role", http.MethodPost, "/roles/{roleId}/preview",
		"What an edit would give and take away, and from whom", ok, roles), h.previewRole)
	huma.Register(api, ws("update-role", http.MethodPatch, "/roles/{roleId}",
		"Edit a role; its members get the change on their next request", ok, roles), h.updateRole)
	huma.Register(api, ws("reset-role", http.MethodPost, "/roles/{roleId}/reset",
		"Restore a system role's defaults", ok, roles), h.resetRole)
	huma.Register(api, ws("delete-role", http.MethodDelete, "/roles/{roleId}",
		"Delete a custom role after moving its members", none, roles), h.deleteRole)

	huma.Register(api, ws("list-members", http.MethodGet, "/members", "Workspace members", ok), h.listMembers)
	huma.Register(api, ws("get-member", http.MethodGet, "/members/{userId}",
		"One member's role, reach and permissions", ok), h.getMember)
	huma.Register(api, ws("preview-member-role", http.MethodPost, "/members/{userId}/role/preview",
		"What a role change would give and take away", ok, roles), h.previewMemberRole)
	huma.Register(api, ws("change-member-role", http.MethodPut, "/members/{userId}/role",
		"Move a member to another role", none, roles), h.changeMemberRole)
	huma.Register(api, ws("update-member-access", http.MethodPatch, "/members/{userId}/access",
		"Change a member's chat scope, teams or WhatsApp numbers", none, roles), h.updateMemberAccess)
	huma.Register(api, ws("add-member-override", http.MethodPost, "/members/{userId}/overrides",
		"Give or take away one permission for one member", none, roles), h.addOverride)
	huma.Register(api, ws("revoke-member-override", http.MethodDelete, "/members/{userId}/overrides/{overrideId}",
		"Remove an extra or removed permission", none, roles), h.revokeOverride)
	huma.Register(api, ws("suspend-member", http.MethodPost, "/members/{userId}/suspend",
		"Pause a member's access; their seat stays reserved", none, remove), h.suspend)
	huma.Register(api, ws("restore-member", http.MethodPost, "/members/{userId}/restore",
		"Give a suspended member their access back", none, remove), h.restore)
	huma.Register(api, ws("remove-member", http.MethodPost, "/members/{userId}/remove",
		"Remove a member from the workspace", none, remove), h.remove)

	huma.Register(api, ws("list-teams", http.MethodGet, "/teams", "Teams and their members", ok), h.listTeams)
	huma.Register(api, ws("create-team", http.MethodPost, "/teams", "Create a team", created, roles), h.createTeam)
	huma.Register(api, ws("update-team", http.MethodPatch, "/teams/{teamId}", "Edit a team", ok, roles), h.updateTeam)
	huma.Register(api, ws("set-team-members", http.MethodPut, "/teams/{teamId}/members",
		"Replace a team's members", ok, roles), h.setTeamMembers)
	huma.Register(api, ws("delete-team", http.MethodDelete, "/teams/{teamId}", "Delete a team", none, roles), h.deleteTeam)

	huma.Register(api, ws("list-invitations", http.MethodGet, "/invitations",
		"Pending invites and those that ended recently", ok, invite), h.listInvitations)
	huma.Register(api, ws("create-invitations", http.MethodPost, "/invitations",
		"Invite people by email", created, invite), h.invite)
	huma.Register(api, ws("resend-invitation", http.MethodPost, "/invitations/{invitationId}/resend",
		"Send a new link; the old one stops working", none, invite), h.resendInvitation)
	huma.Register(api, ws("revoke-invitation", http.MethodDelete, "/invitations/{invitationId}",
		"Cancel a pending invite", none, invite), h.revokeInvitation)

	// The invite link works before sign-in; accepting needs it.
	public := func(id, path, summary string, status int, mw ...Middleware) huma.Operation {
		op := huma.Operation{
			OperationID: id, Method: http.MethodPost, Path: "/v1/invitations" + path, Summary: summary,
			Tags: []string{"team"}, DefaultStatus: status, Middlewares: mw,
		}
		if len(mw) > 0 {
			op.Security = security
		}
		return op
	}
	huma.Register(api, public("preview-invitation", "/preview",
		"What an invite link offers, before signing in", ok), h.previewInvitation)
	huma.Register(api, public("accept-invitation", "/accept",
		"Join the workspace as the signed-in user", ok, g.Auth), h.acceptInvitation)
	huma.Register(api, public("request-invitation-renewal", "/request-renewal",
		"Ask the inviter for a new link after it expired", none), h.requestRenewal)

	huma.Register(api, ws("list-access-log", http.MethodGet, "/access-log",
		"Every change to who can do what, newest first", ok, roles), h.accessLog)
	huma.Register(api, ws("get-access-log-summary", http.MethodGet, "/access-log/summary",
		"The last 30 days at a glance", ok, roles), h.accessLogSummary)
}
