package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
)

// ---------- roles ----------

func (h *Handler) listRoles(ctx context.Context, in *dto.WorkspaceInput) (*dto.RolesOutput, error) {
	roles, err := h.svc.ListRoles(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.RolesOutput{}
	out.Body.Roles = make([]dto.RoleDTO, len(roles))
	for i, r := range roles {
		out.Body.Roles[i] = dto.ToRoleDTO(r)
	}
	return out, nil
}

func (h *Handler) createRole(ctx context.Context, in *dto.CreateRoleInput) (*dto.RoleOutput, error) {
	r, err := h.svc.CreateRole(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.Body.Input())
	if err != nil {
		return nil, err
	}
	return &dto.RoleOutput{Body: dto.ToRoleDTO(r)}, nil
}

func (h *Handler) previewRole(ctx context.Context, in *dto.UpdateRoleInput) (*dto.ChangeOutput, error) {
	c, err := h.svc.PreviewRole(ctx, in.WorkspaceID, in.RoleID, in.Body.Input())
	if err != nil {
		return nil, err
	}
	return &dto.ChangeOutput{Body: dto.ToChangeDTO(c)}, nil
}

func (h *Handler) updateRole(ctx context.Context, in *dto.UpdateRoleInput) (*dto.RoleOutput, error) {
	r, err := h.svc.UpdateRole(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.RoleID, in.Body.Input())
	if err != nil {
		return nil, err
	}
	return &dto.RoleOutput{Body: dto.ToRoleDTO(r)}, nil
}

func (h *Handler) resetRole(ctx context.Context, in *dto.ResetRoleInput) (*dto.RoleOutput, error) {
	r, err := h.svc.ResetRole(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.RoleID)
	if err != nil {
		return nil, err
	}
	return &dto.RoleOutput{Body: dto.ToRoleDTO(r)}, nil
}

func (h *Handler) deleteRole(ctx context.Context, in *dto.DeleteRoleInput) (*struct{}, error) {
	return nil, h.svc.DeleteRole(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.RoleID, in.MoveToRoleID)
}
