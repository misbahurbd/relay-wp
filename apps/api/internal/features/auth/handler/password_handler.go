package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/dto"
)

func (h *Handler) forgotPassword(ctx context.Context, in *dto.EmailInput) (*dto.MessageOutput, error) {
	if err := h.svc.ForgotPassword(ctx, in.Body.Email, meta(ctx, in.RequestHeaders, "")); err != nil {
		return nil, err
	}
	return dto.Message202(checkInbox), nil
}

func (h *Handler) resetPassword(ctx context.Context, in *dto.ResetPasswordInput) (*struct{}, error) {
	return nil, h.svc.ResetPassword(ctx, in.Body.Token, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) changePassword(ctx context.Context, in *dto.ChangePasswordInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.ChangePassword(ctx, uid, sid, in.Body.CurrentPassword, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) setPassword(ctx context.Context, in *dto.SetPasswordInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.SetPassword(ctx, uid, sid, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
}
