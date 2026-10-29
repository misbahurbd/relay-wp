package team

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

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
