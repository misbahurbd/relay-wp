package team

import (
	"context"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

func (h *Handler) leave(ctx context.Context, in *leaveInput) (*struct{}, error) {
	return nil, h.svc.Leave(ctx, actor(ctx), meta(ctx, in.metaHeaders))
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

func (h *Handler) suspend(ctx context.Context, in *suspendInput) (*struct{}, error) {
	return nil, h.svc.Suspend(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.Body.Reason)
}

func (h *Handler) restore(ctx context.Context, in *memberActionInput) (*struct{}, error) {
	return nil, h.svc.Restore(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID)
}

func (h *Handler) remove(ctx context.Context, in *removeInput) (*struct{}, error) {
	return nil, h.svc.Remove(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, RemoveInput{ConfirmName: in.Body.ConfirmName})
}
