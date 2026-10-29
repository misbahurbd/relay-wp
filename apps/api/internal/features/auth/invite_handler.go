package auth

import (
	"context"
	"errors"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func (h *Handler) inviteSignup(ctx context.Context, in *inviteSignupInput) (*inviteSignupOutput, error) {
	b := in.Body
	t, workspaceID, err := h.svc.SignupFromInvite(ctx, InviteSignupInput{
		Token: b.Token, Password: b.Password, FirstName: b.FirstName, LastName: b.LastName, Timezone: b.Timezone,
		AcceptTerms: b.AcceptTerms,
	}, meta(ctx, in.RequestHeaders, clientWeb))
	if t.AccessToken == "" {
		return nil, err
	}
	out := &inviteSignupOutput{SetCookie: h.tokenResponse(t).SetCookie}
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
