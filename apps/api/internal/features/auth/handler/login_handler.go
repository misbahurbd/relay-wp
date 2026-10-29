package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/dto"
)

func (h *Handler) login(ctx context.Context, in *dto.LoginInput) (*dto.TokenOutput, error) {
	t, err := h.svc.Login(ctx, in.Body.Email, in.Body.Password, meta(ctx, in.RequestHeaders, in.Body.Client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}
