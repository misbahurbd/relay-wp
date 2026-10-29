package team

import (
	"context"
)

// ---------- roles ----------

func (h *Handler) listRoles(ctx context.Context, in *workspaceInput) (*rolesOutput, error) {
	roles, err := h.svc.ListRoles(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &rolesOutput{}
	out.Body.Roles = make([]roleDTO, len(roles))
	for i, r := range roles {
		out.Body.Roles[i] = toRoleDTO(r)
	}
	return out, nil
}

func (h *Handler) createRole(ctx context.Context, in *createRoleInput) (*roleOutput, error) {
	r, err := h.svc.CreateRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.Body.input())
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) previewRole(ctx context.Context, in *updateRoleInput) (*changeOutput, error) {
	c, err := h.svc.PreviewRole(ctx, in.WorkspaceID, in.RoleID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &changeOutput{Body: toChangeDTO(c)}, nil
}

func (h *Handler) updateRole(ctx context.Context, in *updateRoleInput) (*roleOutput, error) {
	r, err := h.svc.UpdateRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) resetRole(ctx context.Context, in *resetRoleInput) (*roleOutput, error) {
	r, err := h.svc.ResetRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID)
	if err != nil {
		return nil, err
	}
	return &roleOutput{Body: toRoleDTO(r)}, nil
}

func (h *Handler) deleteRole(ctx context.Context, in *deleteRoleInput) (*struct{}, error) {
	return nil, h.svc.DeleteRole(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.RoleID, in.MoveToRoleID)
}
