package handler

import (
	"context"
	"errors"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/service"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func (h *Handler) inviteSignup(ctx context.Context, in *dto.InviteSignupInput) (*dto.InviteSignupOutput, error) {
	b := in.Body
	t, workspaceID, err := h.svc.SignupFromInvite(ctx, service.InviteSignupInput{
		Token: b.Token, Password: b.Password, FirstName: b.FirstName, LastName: b.LastName, Timezone: b.Timezone,
		AcceptTerms: b.AcceptTerms,
	}, meta(ctx, in.RequestHeaders, model.ClientWeb))
	if t.AccessToken == "" {
		return nil, err
	}
	out := &dto.InviteSignupOutput{SetCookie: h.tokenResponse(t).SetCookie}
	if err != nil {
		msg := "Your account is ready, but joining the workspace failed. Open the invite link again."
		var ae *apperr.Error
		if errors.As(err, &ae) {
			msg = ae.Message
		}
		out.Body.JoinError = &msg
		return out, nil
	}
	out.Body.WorkspaceID = &workspaceID
	return out, nil
}
