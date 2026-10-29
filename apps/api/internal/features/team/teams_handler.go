package team

import (
	"context"
)

// ---------- teams ----------

func (h *Handler) listTeams(ctx context.Context, in *workspaceInput) (*teamsOutput, error) {
	teams, err := h.svc.ListTeams(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &teamsOutput{}
	out.Body.Teams = make([]teamDTO, len(teams))
	for i, t := range teams {
		out.Body.Teams[i] = toTeamDTO(t)
	}
	return out, nil
}

func (h *Handler) createTeam(ctx context.Context, in *createTeamInput) (*teamOutput, error) {
	t, err := h.svc.CreateTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.Body.input())
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) updateTeam(ctx context.Context, in *updateTeamInput) (*teamOutput, error) {
	t, err := h.svc.UpdateTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID, in.Body.input())
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) setTeamMembers(ctx context.Context, in *teamMembersInput) (*teamOutput, error) {
	t, err := h.svc.SetTeamMembers(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID, in.Body.UserIDs)
	if err != nil {
		return nil, err
	}
	return &teamOutput{Body: toTeamDTO(t)}, nil
}

func (h *Handler) deleteTeam(ctx context.Context, in *deleteTeamInput) (*struct{}, error) {
	return nil, h.svc.DeleteTeam(ctx, actor(ctx), meta(ctx, in.metaHeaders), in.TeamID)
}
