package auth

import (
	"context"
)

func (h *Handler) signup(ctx context.Context, in *signupInput) (*messageOutput, error) {
	err := h.svc.Signup(ctx, SignupInput{
		Email: in.Body.Email, Password: in.Body.Password,
		FirstName: in.Body.FirstName, LastName: in.Body.LastName, Timezone: in.Body.Timezone,
		AcceptTerms: in.Body.AcceptTerms,
	}, meta(ctx, in.RequestHeaders, clientWeb))
	if err != nil {
		return nil, err
	}
	return message202("Check your inbox to verify your email address."), nil
}

func (h *Handler) verifyEmail(ctx context.Context, in *verifyEmailInput) (*struct{}, error) {
	return nil, h.svc.VerifyEmail(ctx, in.Body.Token, in.Body.Password, meta(ctx, in.RequestHeaders, clientWeb))
}

func (h *Handler) verifyEmailCode(ctx context.Context, in *verifyCodeInput) (*tokenOutput, error) {
	t, err := h.svc.VerifyEmailCode(ctx, in.Body.Email, in.Body.Code, in.Body.Password, meta(ctx, in.RequestHeaders, in.Body.Client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) resendVerification(ctx context.Context, in *emailInput) (*messageOutput, error) {
	if err := h.svc.ResendVerification(ctx, in.Body.Email, meta(ctx, in.RequestHeaders, clientWeb)); err != nil {
		return nil, err
	}
	return message202(checkInbox), nil
}
