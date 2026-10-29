package team

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ListTeams returns the workspace's teams by name.
func (s *Service) ListTeams(ctx context.Context, workspaceID uuid.UUID) ([]Team, error) {
	rows, err := newRepository(s.pool).ListTeams(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	out := make([]Team, len(rows))
	for i, t := range rows {
		out[i] = teamFromRow(gen.GetTeamRow(t))
	}
	return out, nil
}

func (s *Service) getTeam(ctx context.Context, db database.DBTX, workspaceID, teamID uuid.UUID) (Team, error) {
	row, err := newRepository(db).GetTeam(ctx, gen.GetTeamParams{WorkspaceID: workspaceID, TeamID: teamID})
	if isNoRows(err) {
		return Team{}, ErrTeamNotFound
	}
	if err != nil {
		return Team{}, fmt.Errorf("get team: %w", err)
	}
	return teamFromRow(row), nil
}

// CreateTeam adds a team.
func (s *Service) CreateTeam(ctx context.Context, actor authctx.Access, meta Meta, in Details) (Team, error) {
	in = normalizeTeam(in)
	if err := s.validateTeam(ctx, actor.WorkspaceID, in); err != nil {
		return Team{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Team{}, fmt.Errorf("team id: %w", err)
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		err := r.CreateTeam(ctx, gen.CreateTeamParams{
			ID: id, WorkspaceID: actor.WorkspaceID, Name: in.Name, Description: in.Description, Color: in.Color,
			NumberID: in.NumberID, RoutingMethod: in.RoutingMethod,
		})
		if isUnique(err, "teams_name_idx") {
			return ErrTeamName
		}
		if err != nil {
			return fmt.Errorf("create team: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionTeamChanged, target: id.String(),
			after: map[string]any{"name": in.Name}, note: "Team created",
		}, meta)
	})
	if err != nil {
		return Team{}, err
	}
	return s.getTeam(ctx, s.pool, actor.WorkspaceID, id)
}

// UpdateTeam edits a team's details.
func (s *Service) UpdateTeam(ctx context.Context, actor authctx.Access, meta Meta, teamID uuid.UUID, in Details) (Team, error) {
	in = normalizeTeam(in)
	if err := s.validateTeam(ctx, actor.WorkspaceID, in); err != nil {
		return Team{}, err
	}
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		before, err := s.getTeam(ctx, tx, actor.WorkspaceID, teamID)
		if err != nil {
			return err
		}
		_, err = r.UpdateTeam(ctx, gen.UpdateTeamParams{
			WorkspaceID: actor.WorkspaceID, TeamID: teamID, Name: in.Name, Description: in.Description,
			Color: in.Color, NumberID: in.NumberID, RoutingMethod: in.RoutingMethod,
		})
		if isUnique(err, "teams_name_idx") {
			return ErrTeamName
		}
		if err != nil {
			return fmt.Errorf("update team: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionTeamChanged, target: teamID.String(),
			before: map[string]any{"name": before.Name, "routing_method": before.RoutingMethod},
			after:  map[string]any{"name": in.Name, "routing_method": in.RoutingMethod},
		}, meta)
	})
	if err != nil {
		return Team{}, err
	}
	return s.getTeam(ctx, s.pool, actor.WorkspaceID, teamID)
}

// SetTeamMembers replaces a team's members. Members who join or leave see the
// change on their next request.
func (s *Service) SetTeamMembers(ctx context.Context, actor authctx.Access, meta Meta, teamID uuid.UUID, userIDs []uuid.UUID) (Team, error) {
	userIDs = uniqueIDs(userIDs)
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		before, err := s.getTeam(ctx, tx, actor.WorkspaceID, teamID)
		if err != nil {
			return err
		}
		changed := symmetricDiff(before.MemberIDs, userIDs)
		if len(changed) == 0 {
			return nil
		}
		err = r.SetTeamMembers(ctx, gen.SetTeamMembersParams{WorkspaceID: actor.WorkspaceID, TeamID: teamID, UserIds: userIDs})
		if isForeignKey(err) {
			return ErrInvalidInput.WithDetails(apperr.Detail{Field: "user_ids", Message: "Choose members of this workspace"})
		}
		if err != nil {
			return fmt.Errorf("set team members: %w", err)
		}
		if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: actor.WorkspaceID, UserIds: changed}); err != nil {
			return fmt.Errorf("bump access: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionTeamChanged, target: teamID.String(),
			before: map[string]any{"user_ids": before.MemberIDs}, after: map[string]any{"user_ids": userIDs},
			note: fmt.Sprintf("%s: %d members changed", before.Name, len(changed)),
		}, meta)
	})
	if err != nil {
		return Team{}, err
	}
	return s.getTeam(ctx, s.pool, actor.WorkspaceID, teamID)
}

// DeleteTeam removes a team. Its members lose its chats on their next request.
func (s *Service) DeleteTeam(ctx context.Context, actor authctx.Access, meta Meta, teamID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		team, err := s.getTeam(ctx, tx, actor.WorkspaceID, teamID)
		if err != nil {
			return err
		}
		if _, err := r.DeleteTeam(ctx, gen.DeleteTeamParams{WorkspaceID: actor.WorkspaceID, TeamID: teamID}); err != nil {
			return fmt.Errorf("delete team: %w", err)
		}
		if len(team.MemberIDs) > 0 {
			if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: actor.WorkspaceID, UserIds: team.MemberIDs}); err != nil {
				return fmt.Errorf("bump access: %w", err)
			}
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionTeamChanged, target: teamID.String(),
			before: map[string]any{"name": team.Name, "user_ids": team.MemberIDs}, note: "Team deleted",
		}, meta)
	})
}

func normalizeTeam(in Details) Details {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Color == "" {
		in.Color = "blue"
	}
	if in.RoutingMethod == "" {
		in.RoutingMethod = "round_robin"
	}
	return in
}

func (s *Service) validateTeam(ctx context.Context, workspaceID uuid.UUID, in Details) error {
	var details []apperr.Detail
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 50 {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter a team name of up to 50 characters"})
	}
	if utf8.RuneCountInString(in.Description) > 200 {
		details = append(details, apperr.Detail{Field: "description", Message: "Keep the description under 200 characters"})
	}
	if !slices.Contains(colors, in.Color) {
		details = append(details, apperr.Detail{Field: "color", Message: "Choose one of the colors"})
	}
	if !slices.Contains(routingMethods, in.RoutingMethod) {
		details = append(details, apperr.Detail{Field: "routing_method", Message: "Choose how chats are shared out"})
	}
	if in.NumberID != nil {
		n, err := newRepository(s.pool).CountWorkspaceNumbers(ctx, gen.CountWorkspaceNumbersParams{
			WorkspaceID: workspaceID, NumberIds: []uuid.UUID{*in.NumberID},
		})
		if err != nil {
			return fmt.Errorf("check number: %w", err)
		}
		if n != 1 {
			details = append(details, apperr.Detail{Field: "number_id", Message: "Choose a number connected to this workspace"})
		}
	}
	if len(details) > 0 {
		return ErrInvalidInput.WithDetails(details...)
	}
	return nil
}

// symmetricDiff returns the IDs in exactly one of a and b.
func symmetricDiff(a, b []uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, id := range a {
		if !slices.Contains(b, id) {
			out = append(out, id)
		}
	}
	for _, id := range b {
		if !slices.Contains(a, id) {
			out = append(out, id)
		}
	}
	return out
}

func teamFromRow(r gen.GetTeamRow) Team {
	return Team{
		ID: r.ID, Name: r.Name, Description: r.Description, Color: r.Color, NumberID: r.NumberID,
		RoutingMethod: r.RoutingMethod, MemberIDs: r.MemberIds,
	}
}
