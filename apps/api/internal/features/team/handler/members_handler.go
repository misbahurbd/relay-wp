package handler

import (
	"context"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

func (h *Handler) leave(ctx context.Context, in *dto.LeaveInput) (*struct{}, error) {
	return nil, h.svc.Leave(ctx, actor(ctx), meta(ctx, in.MetaHeaders))
}

// ---------- members ----------

func (h *Handler) listMembers(ctx context.Context, in *dto.ListMembersInput) (*dto.MembersOutput, error) {
	f := model.MemberFilter{
		RoleID: dto.OptionalID(in.RoleID), TeamID: dto.OptionalID(in.TeamID), Status: dto.Optional(in.Status),
		Query: dto.Optional(in.Query), PageSize: in.PageSize,
	}
	if in.Cursor != "" {
		t, id, ok := dto.DecodeCursor(in.Cursor)
		if !ok {
			return nil, errBadCursor
		}
		f.After = &model.MemberCursor{JoinedAt: t, UserID: id}
	}
	members, next, err := h.svc.ListMembers(ctx, in.WorkspaceID, f)
	if err != nil {
		return nil, err
	}
	out := &dto.MembersOutput{}
	out.Body.Members = make([]dto.MemberDTO, len(members))
	for i, m := range members {
		out.Body.Members[i] = dto.ToMemberDTO(m)
	}
	if next != nil {
		c := dto.EncodeCursor(next.JoinedAt, next.UserID)
		out.Body.NextCursor = &c
	}
	return out, nil
}

func (h *Handler) getMember(ctx context.Context, in *dto.MemberPath) (*dto.MemberOutput, error) {
	d, err := h.svc.GetMember(ctx, in.WorkspaceID, in.UserID, actor(ctx).Can(authctx.PermTeamRoles))
	if err != nil {
		return nil, err
	}
	return &dto.MemberOutput{Body: dto.ToMemberDetailDTO(d, time.Now())}, nil
}

func (h *Handler) previewMemberRole(ctx context.Context, in *dto.MemberRoleInput) (*dto.ChangeOutput, error) {
	c, err := h.svc.PreviewMemberRole(ctx, in.WorkspaceID, in.UserID, in.Body.RoleID)
	if err != nil {
		return nil, err
	}
	return &dto.ChangeOutput{Body: dto.ToChangeDTO(c)}, nil
}

func (h *Handler) changeMemberRole(ctx context.Context, in *dto.MemberRoleInput) (*struct{}, error) {
	return nil, h.svc.ChangeMemberRole(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, in.Body.RoleID)
}

func (h *Handler) updateMemberAccess(ctx context.Context, in *dto.MemberAccessInput) (*struct{}, error) {
	a := model.AccessInput{ChatScope: in.Body.ChatScope}
	if in.Body.TeamIDs != nil {
		a.SetTeams, a.TeamIDs = true, *in.Body.TeamIDs
	}
	if in.Body.NumberIDs != nil {
		a.SetNumber, a.NumberIDs = true, *in.Body.NumberIDs
	}
	return nil, h.svc.UpdateMemberAccess(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, a)
}

func (h *Handler) suspend(ctx context.Context, in *dto.SuspendInput) (*struct{}, error) {
	return nil, h.svc.Suspend(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, in.Body.Reason)
}

func (h *Handler) restore(ctx context.Context, in *dto.MemberActionInput) (*struct{}, error) {
	return nil, h.svc.Restore(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID)
}

func (h *Handler) remove(ctx context.Context, in *dto.RemoveInput) (*struct{}, error) {
	return nil, h.svc.Remove(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, model.RemoveInput{ConfirmName: in.Body.ConfirmName})
}
