package team

import (
	"context"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Handler turns HTTP requests into service calls.
type Handler struct {
	svc *Service
}

// NewHandler builds the team HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

var errBadCursor = apperr.Validation("invalid_cursor", "That page link has expired. Start from the first page.")

// actor is set by middleware.RequireWorkspace on every workspace route here.
func actor(ctx context.Context) authctx.Access {
	a, _ := authctx.AccessFrom(ctx)
	return a
}

func meta(ctx context.Context, h metaHeaders) Meta {
	return Meta{IP: middleware.ClientIP(ctx), UserAgent: h.UserAgent}
}

// ---------- access ----------

func (h *Handler) myAccess(ctx context.Context, _ *workspaceInput) (*accessOutput, error) {
	return &accessOutput{Body: toAccessDTO(actor(ctx))}, nil
}

func (h *Handler) setAvailability(ctx context.Context, in *availabilityInput) (*struct{}, error) {
	return nil, h.svc.SetAvailability(ctx, actor(ctx), in.Body.Availability)
}

func (h *Handler) leave(ctx context.Context, in *leaveInput) (*struct{}, error) {
	return nil, h.svc.Leave(ctx, actor(ctx), meta(ctx, in.metaHeaders))
}

func (h *Handler) permissions(ctx context.Context, _ *struct{}) (*permissionsOutput, error) {
	list, err := h.svc.Permissions(ctx)
	if err != nil {
		return nil, err
	}
	out := &permissionsOutput{}
	out.Body.Permissions = make([]permissionDTO, len(list))
	for i, p := range list {
		out.Body.Permissions[i] = permissionDTO(p)
	}
	return out, nil
}

func (h *Handler) seats(ctx context.Context, in *workspaceInput) (*seatsOutput, error) {
	s, err := h.svc.Seats(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &seatsOutput{Body: seatsDTO(s)}, nil
}

// ---------- roles ----------

func (h *Handler) listRoles(ctx context.Context, in *workspaceInput) (*rolesOutput, error) {
	roles, err := h.svc.ListRoles(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &rolesOutput{}
	out.Body.Roles = make([]roleDTO, len(roles))
	for i, r := range roles {
		out.Body.Roles[i] = toRoleDTO(r)
	}
	return out, nil
}

func (h *Handler) createRole(ctx context.Context, in *createRoleInput) (*roleOutput, error) {
	r, err := h.svc.CreateRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.Body.input())
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) previewRole(ctx context.Context, in *updateRoleInput) (*changeOutput, error) {
	c, err := h.svc.PreviewRole(ctx, in.WorkspaceID, in.RoleID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &changeOutput{Body: toChangeDTO(c)}, nil
}

func (h *Handler) updateRole(ctx context.Context, in *updateRoleInput) (*roleOutput, error) {
	r, err := h.svc.UpdateRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) resetRole(ctx context.Context, in *resetRoleInput) (*roleOutput, error) {
	r, err := h.svc.ResetRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID)
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) deleteRole(ctx context.Context, in *deleteRoleInput) (*struct{}, error) {
	return nil, h.svc.DeleteRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID, in.MoveToRoleID)
}

// ---------- members ----------

func (h *Handler) listMembers(ctx context.Context, in *listMembersInput) (*membersOutput, error) {
	f := MemberFilter{
		RoleID: optionalID(in.RoleID), TeamID: optionalID(in.TeamID), Status: optional(in.Status),
		Query: optional(in.Query), PageSize: in.PageSize,
	}
	if in.Cursor != "" {
		t, id, ok := decodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		f.After = &MemberCursor{JoinedAt: t, UserID: id}
	}
	members, next, err := h.svc.ListMembers(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &membersOutput{}
	out.Body.Members = make([]MemberDTO, len(members))
	for i, m := range members {
		out.Body.Members[i] = toMemberDTO(m)
	}
	if next != nil {
		c := encodeCursor(next.JoinedAt, next.UserID)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) getMember(ctx context.Context, in *memberPath) (*memberOutput, error) {
	d, err := h.svc.GetMember(ctx, in.WorkspaceID, in.UserID, actor(ctx).Can(authctx.PermTeamRoles))
	if err != nil {
		return nil, err
	}
	return &memberOutput{Body: toMemberDetailDTO(d, time.Now())}, nil
}

func (h *Handler) previewMemberRole(ctx context.Context, in *memberRoleInput) (*changeOutput, error) {
	c, err := h.svc.PreviewMemberRole(ctx, in.WorkspaceID, in.UserID, in.Body.RoleID)
	if err != nil {
		return nil, err
	}
	return &changeOutput{Body: toChangeDTO(c)}, nil
}

func (h *Handler) changeMemberRole(ctx context.Context, in *memberRoleInput) (*struct{}, error) {
	return nil, h.svc.ChangeMemberRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.Body.RoleID)
}

func (h *Handler) updateMemberAccess(ctx context.Context, in *memberAccessInput) (*struct{}, error) {
	a := AccessInput{ChatScope: in.Body.ChatScope}
	if in.Body.TeamIDs != nil {
		a.SetTeams, a.TeamIDs = true, *in.Body.TeamIDs
	}
	if in.Body.NumberIDs != nil {
		a.SetNumber, a.NumberIDs = true, *in.Body.NumberIDs
	}
	return nil, h.svc.UpdateMemberAccess(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, a)
}

func (h *Handler) addOverride(ctx context.Context, in *addOverrideInput) (*struct{}, error) {
	return nil, h.svc.AddOverride(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.Body.input())
}

func (h *Handler) revokeOverride(ctx context.Context, in *revokeOverrideInput) (*struct{}, error) {
	return nil, h.svc.RevokeOverride(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.OverrideID)
}

func (h *Handler) suspend(ctx context.Context, in *suspendInput) (*struct{}, error) {
	return nil, h.svc.Suspend(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.Body.Reason)
}

func (h *Handler) restore(ctx context.Context, in *memberActionInput) (*struct{}, error) {
	return nil, h.svc.Restore(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID)
}

func (h *Handler) remove(ctx context.Context, in *removeInput) (*struct{}, error) {
	return nil, h.svc.Remove(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, RemoveInput{ConfirmName: in.Body.ConfirmName})
}

// ---------- teams ----------

func (h *Handler) listTeams(ctx context.Context, in *workspaceInput) (*teamsOutput, error) {
	teams, err := h.svc.ListTeams(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &teamsOutput{}
	out.Body.Teams = make([]teamDTO, len(teams))
	for i, t := range teams {
		out.Body.Teams[i] = toTeamDTO(t)
	}
	return out, nil
}

func (h *Handler) createTeam(ctx context.Context, in *createTeamInput) (*teamOutput, error) {
	t, err := h.svc.CreateTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.Body.input())
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) updateTeam(ctx context.Context, in *updateTeamInput) (*teamOutput, error) {
	t, err := h.svc.UpdateTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) setTeamMembers(ctx context.Context, in *teamMembersInput) (*teamOutput, error) {
	t, err := h.svc.SetTeamMembers(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID, in.Body.UserIDs)
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) deleteTeam(ctx context.Context, in *deleteTeamInput) (*struct{}, error) {
	return nil, h.svc.DeleteTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID)
}

// ---------- invitations ----------

func (h *Handler) listInvitations(ctx context.Context, in *workspaceInput) (*invitationsOutput, error) {
	list, err := h.svc.ListInvitations(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &invitationsOutput{}
	out.Body.Invitations = make([]invitationDTO, len(list))
	for i, inv := range list {
		out.Body.Invitations[i] = toInvitationDTO(inv)
	}
	return out, nil
}

func (h *Handler) invite(ctx context.Context, in *inviteInput) (*invitationsOutput, error) {
	b := in.Body
	input := InviteInput{
		Emails: b.Emails, RoleID: b.RoleID, TeamIDs: b.TeamIDs, NumberIDs: b.NumberIDs, ChatScope: b.ChatScope,
		PersonalMessage: b.PersonalMessage, ExpiresIn: b.ExpiresIn, Require2FA: b.Require2FA,
	}
	for _, o := range b.Overrides {
		input.Overrides = append(input.Overrides, o.input())
	}
	list, err := h.svc.Invite(ctx, actor(ctx), meta(ctx, in.metaHeaders), input)
	if err != nil {
		return nil, err
	}
	out := &invitationsOutput{}
	out.Body.Invitations = make([]invitationDTO, len(list))
	for i, inv := range list {
		out.Body.Invitations[i] = toInvitationDTO(inv)
	}
	return out, nil
}

func (h *Handler) resendInvitation(ctx context.Context, in *invitationActionInput) (*struct{}, error) {
	return nil, h.svc.ResendInvitation(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.InvitationID)
}

func (h *Handler) revokeInvitation(ctx context.Context, in *invitationActionInput) (*struct{}, error) {
	return nil, h.svc.RevokeInvitation(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.InvitationID)
}

func (h *Handler) previewInvitation(ctx context.Context, in *tokenInput) (*previewOutput, error) {
	p, err := h.svc.PreviewInvitation(ctx, in.Body.Token)
	if err != nil {
		return nil, err
	}
	return &previewOutput{Body: previewDTO{
		State: p.State, WorkspaceID: p.WorkspaceID, WorkspaceName: p.WorkspaceName, Email: p.Email, RoleName: p.RoleName,
		InvitedByName: p.InvitedByName, InvitedByRole: p.InvitedByRole, ExpiresAt: p.ExpiresAt, Message: p.Message,
		CanDo: nonNil(p.CanDo), Permissions: nonNil(p.PermissionKeys), TeamNames: nonNil(p.TeamNames),
		NumberLabels: nonNil(p.NumberLabels),
	}}, nil
}

func (h *Handler) acceptInvitation(ctx context.Context, in *tokenInput) (*acceptOutput, error) {
	userID, _ := authctx.UserID(ctx)
	ws, err := h.svc.AcceptInvitation(ctx, in.Body.Token, userID, meta(ctx, in.metaHeaders))
	if err != nil {
		return nil, err
	}
	out := &acceptOutput{}
	out.Body.WorkspaceID = ws
	return out, nil
}

func (h *Handler) requestRenewal(ctx context.Context, in *tokenInput) (*struct{}, error) {
	return nil, h.svc.RequestRenewal(ctx, in.Body.Token)
}

// ---------- access log ----------

func (h *Handler) accessLog(ctx context.Context, in *listLogInput) (*logOutput, error) {
	f := LogFilter{Action: optional(in.Action), SubjectID: optionalID(in.SubjectID), Since: in.Since, PageSize: in.PageSize}
	if in.Cursor != "" {
		t, id, ok := decodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		f.Before = &LogCursor{CreatedAt: t, ID: id}
	}
	entries, next, err := h.svc.ListAccessLog(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &logOutput{}
	out.Body.Entries = make([]logDTO, len(entries))
	for i, e := range entries {
		out.Body.Entries[i] = toLogDTO(e)
	}
	if next != nil {
		c := encodeCursor(next.CreatedAt, next.ID)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) accessLogSummary(ctx context.Context, in *workspaceInput) (*logSummaryOutput, error) {
	sum, err := h.svc.AccessLogSummary(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &logSummaryOutput{}
	out.Body.Events, out.Body.PermissionChanges = sum.Events, sum.PermissionChanges
	out.Body.Expiring = make([]expiringDTO, len(sum.Expiring))
	for i, e := range sum.Expiring {
		out.Body.Expiring[i] = expiringDTO(e)
	}
	out.Body.Alerts = make([]loginAlertDTO, len(sum.Alerts))
	for i, a := range sum.Alerts {
		out.Body.Alerts[i] = loginAlertDTO(a)
	}
	return out, nil
}
