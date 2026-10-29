package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
)

func (h *Handler) signup(ctx context.Context, in *dto.SignupInput) (*dto.MessageOutput, error) {
	err := h.svc.Signup(ctx, model.SignupInput{
		Email: in.Body.Email, Password: in.Body.Password,
		FirstName: in.Body.FirstName, LastName: in.Body.LastName, Timezone: in.Body.Timezone,
		AcceptTerms: in.Body.AcceptTerms,
	}, meta(ctx, in.RequestHeaders, model.ClientWeb))
	if err != nil {
		return nil, err
	}
	return dto.Message202("Check your inbox to verify your email address."), nil
}

func (h *Handler) verifyEmail(ctx context.Context, in *dto.VerifyEmailInput) (*struct{}, error) {
	return nil, h.svc.VerifyEmail(ctx, in.Body.Token, in.Body.Password, meta(ctx, in.RequestHeaders, model.ClientWeb))
}

func (h *Handler) verifyEmailCode(ctx context.Context, in *dto.VerifyCodeInput) (*dto.TokenOutput, error) {
	t, err := h.svc.VerifyEmailCode(ctx, in.Body.Email, in.Body.Code, in.Body.Password, meta(ctx, in.RequestHeaders, in.Body.Client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) resendVerification(ctx context.Context, in *dto.EmailInput) (*dto.MessageOutput, error) {
	if err := h.svc.ResendVerification(ctx, in.Body.Email, meta(ctx, in.RequestHeaders, model.ClientWeb)); err != nil {
		return nil, err
	}
	return dto.Message202(checkInbox), nil
}
