package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/dto"
)

// ---------- teams ----------

func (h *Handler) listTeams(ctx context.Context, in *dto.WorkspaceInput) (*dto.TeamsOutput, error) {
	teams, err := h.svc.ListTeams(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.TeamsOutput{}
	out.Body.Teams = make([]dto.TeamDTO, len(teams))
	for i, t := range teams {
		out.Body.Teams[i] = dto.ToTeamDTO(t)
	}
	return out, nil
}

func (h *Handler) createTeam(ctx context.Context, in *dto.CreateTeamInput) (*dto.TeamOutput, error) {
	t, err := h.svc.CreateTeam(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.Body.Input())
	if err != nil {
		return nil, err
	}
	return &dto.TeamOutput{Body: dto.ToTeamDTO(t)}, nil
}

func (h *Handler) updateTeam(ctx context.Context, in *dto.UpdateTeamInput) (*dto.TeamOutput, error) {
	t, err := h.svc.UpdateTeam(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.TeamID, in.Body.Input())
	if err != nil {
		return nil, err
	}
	return &dto.TeamOutput{Body: dto.ToTeamDTO(t)}, nil
}

func (h *Handler) setTeamMembers(ctx context.Context, in *dto.TeamMembersInput) (*dto.TeamOutput, error) {
	t, err := h.svc.SetTeamMembers(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.TeamID, in.Body.UserIDs)
	if err != nil {
		return nil, err
	}
	return &dto.TeamOutput{Body: dto.ToTeamDTO(t)}, nil
}

func (h *Handler) deleteTeam(ctx context.Context, in *dto.DeleteTeamInput) (*struct{}, error) {
	return nil, h.svc.DeleteTeam(ctx, actor(ctx), meta(ctx, in.MetaHeaders), in.TeamID)
}
