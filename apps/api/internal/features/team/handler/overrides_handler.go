package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
)

func (h *Handler) addOverride(ctx context.Context, in *dto.AddOverrideInput) (*struct{}, error) {
	return nil, h.svc.AddOverride(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, in.Body.Input())
}

func (h *Handler) revokeOverride(ctx context.Context, in *dto.RevokeOverrideInput) (*struct{}, error) {
	return nil, h.svc.RevokeOverride(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.UserID, in.OverrideID)
}
