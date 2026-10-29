package team

import (
	"context"
)

func (h *Handler) addOverride(ctx context.Context, in *addOverrideInput) (*struct{}, error) {
	return nil, h.svc.AddOverride(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.Body.input())
}

func (h *Handler) revokeOverride(ctx context.Context, in *revokeOverrideInput) (*struct{}, error) {
	return nil, h.svc.RevokeOverride(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.UserID, in.OverrideID)
}
