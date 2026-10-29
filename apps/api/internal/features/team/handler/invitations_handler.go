package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ---------- invitations ----------

func (h *Handler) listInvitations(ctx context.Context, in *dto.WorkspaceInput) (*dto.InvitationsOutput, error) {
	list, err := h.svc.ListInvitations(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.InvitationsOutput{}
	out.Body.Invitations = make([]dto.InvitationDTO, len(list))
	for i, inv := range list {
		out.Body.Invitations[i] = dto.ToInvitationDTO(inv)
	}
	return out, nil
}

func (h *Handler) invite(ctx context.Context, in *dto.InviteInput) (*dto.InvitationsOutput, error) {
	b := in.Body
	input := model.InviteInput{
		Emails: b.Emails, RoleID: b.RoleID, TeamIDs: b.TeamIDs, NumberIDs: b.NumberIDs, ChatScope: b.ChatScope,
		PersonalMessage: b.PersonalMessage, ExpiresIn: b.ExpiresIn, Require2FA: b.Require2FA,
	}
	for _, o := range b.Overrides {
		input.Overrides = append(input.Overrides, o.Input())
	}
	list, err := h.svc.Invite(ctx, actor(ctx), meta(ctx, in.MetaHeaders), input)
	if err != nil {
		return nil, err
	}
	out := &dto.InvitationsOutput{}
	out.Body.Invitations = make([]dto.InvitationDTO, len(list))
	for i, inv := range list {
		out.Body.Invitations[i] = dto.ToInvitationDTO(inv)
	}
	return out, nil
}

func (h *Handler) resendInvitation(ctx context.Context, in *dto.InvitationActionInput) (*struct{}, error) {
	return nil, h.svc.ResendInvitation(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.InvitationID)
}

func (h *Handler) revokeInvitation(ctx context.Context, in *dto.InvitationActionInput) (*struct{}, error) {
	return nil, h.svc.RevokeInvitation(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.InvitationID)
}

func (h *Handler) previewInvitation(ctx context.Context, in *dto.TokenInput) (*dto.PreviewOutput, error) {
	p, err := h.svc.PreviewInvitation(ctx, in.Body.Token)
	if err != nil {
		return nil, err
	}
	return &dto.PreviewOutput{Body: dto.PreviewDTO{
		State: p.State, WorkspaceID: p.WorkspaceID, WorkspaceName: p.WorkspaceName, Email: p.Email, RoleName: p.RoleName,
		InvitedByName: p.InvitedByName, InvitedByRole: p.InvitedByRole, ExpiresAt: p.ExpiresAt, Message: p.Message,
		CanDo: dto.NonNil(p.CanDo), Permissions: dto.NonNil(p.PermissionKeys), TeamNames: dto.NonNil(p.TeamNames),
		NumberLabels: dto.NonNil(p.NumberLabels),
	}}, nil
}

func (h *Handler) acceptInvitation(ctx context.Context, in *dto.TokenInput) (*dto.AcceptOutput, error) {
	userID, _ := authctx.UserID(ctx)
	ws, err := h.svc.AcceptInvitation(ctx, in.Body.Token, userID, meta(ctx, in.MetaHeaders))
	if err != nil {
		return nil, err
	}
	out := &dto.AcceptOutput{}
	out.Body.WorkspaceID = ws
	return out, nil
}

func (h *Handler) requestRenewal(ctx context.Context, in *dto.TokenInput) (*struct{}, error) {
	return nil, h.svc.RequestRenewal(ctx, in.Body.Token)
}
