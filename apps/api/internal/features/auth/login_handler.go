package auth

import (
	"context"
)

func (h *Handler) login(ctx context.Context, in *loginInput) (*tokenOutput, error) {
	t, err := h.svc.Login(ctx, in.Body.Email, in.Body.Password, meta(ctx, in.RequestHeaders, in.Body.Client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}
