package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ListMembers returns one page of members, oldest first, and the cursor for the next page.
func (s *Service) ListMembers(ctx context.Context, workspaceID uuid.UUID, f model.MemberFilter) ([]model.Member, *model.MemberCursor, error) {
	if f.PageSize <= 0 || f.PageSize > 500 {
		f.PageSize = 100
	}
	p := gen.ListMembersParams{
		WorkspaceID: workspaceID, RoleID: f.RoleID, TeamID: f.TeamID, Status: f.Status, Query: f.Query,
		PageSize: f.PageSize + 1,
	}
	if f.After != nil {
		p.AfterJoinedAt, p.AfterUserID = &f.After.JoinedAt, &f.After.UserID
	}
	rows, err := repository.NewRepository(s.pool).ListMembers(ctx, p)
	if err != nil {
		return nil, nil, fmt.Errorf("list members: %w", err)
	}
	var next *model.MemberCursor
	if len(rows) > int(f.PageSize) {
		rows = rows[:f.PageSize]
		last := rows[len(rows)-1]
		next = &model.MemberCursor{JoinedAt: last.JoinedAt, UserID: last.UserID}
	}
	out := make([]model.Member, len(rows))
	for i, r := range rows {
		out[i] = model.Member{
			UserID: r.UserID, Email: r.Email, Name: r.Name, AvatarURL: r.AvatarUrl, RoleID: r.RoleID, RoleName: r.RoleName,
			RoleKey: r.SystemKey, UsesSeat: r.UsesSeat, Status: r.Status, Availability: r.Availability,
			LastActiveAt: r.LastActiveAt, JoinedAt: r.JoinedAt, TeamIDs: r.TeamIds,
		}
	}
	return out, next, nil
}

// GetMember returns the member page. Overrides are included only when
// withOverrides is true (the caller can manage roles).
func (s *Service) GetMember(ctx context.Context, workspaceID, userID uuid.UUID, withOverrides bool) (model.MemberDetail, error) {
	r := repository.NewRepository(s.pool)
	row, err := r.GetMember(ctx, gen.GetMemberParams{WorkspaceID: workspaceID, UserID: userID})
	if repository.IsNoRows(err) {
		return model.MemberDetail{}, model.ErrMemberNotFound
	}
	if err != nil {
		return model.MemberDetail{}, fmt.Errorf("get member: %w", err)
	}
	d := model.MemberDetail{
		Member: model.Member{
			UserID: row.UserID, Email: row.Email, Name: row.Name, AvatarURL: row.AvatarUrl, RoleID: row.RoleID,
			RoleName: row.RoleName, RoleKey: row.SystemKey, UsesSeat: row.UsesSeat, Status: row.Status,
			Availability: row.Availability, LastActiveAt: row.LastActiveAt, JoinedAt: row.JoinedAt, TeamIDs: row.TeamIds,
		},
		ChatScope: row.ChatScope, RoleChatScope: row.RoleChatScope, SuspendedAt: row.SuspendedAt,
		SuspendReason: row.SuspendReason, AccessVersion: row.AccessVersion, NumberIDs: row.NumberIds,
	}
	access, err := r.ResolveAccess(ctx, gen.ResolveAccessParams{WorkspaceID: workspaceID, UserID: userID})
	if err != nil {
		return model.MemberDetail{}, fmt.Errorf("member access: %w", err)
	}
	d.Effective = toAccess(workspaceID, userID, access).Bits.Keys()
	if withOverrides {
		rows, err := r.ListMemberOverrides(ctx, gen.ListMemberOverridesParams{WorkspaceID: workspaceID, UserID: userID})
		if err != nil {
			return model.MemberDetail{}, fmt.Errorf("list overrides: %w", err)
		}
		for _, o := range rows {
			d.Overrides = append(d.Overrides, model.Override(o))
		}
	}
	return d, nil
}

// lockTarget locks the member row and applies the rules every change to
// someone else's access follows: not yourself, and only the owner touches the owner.
func lockTarget(ctx context.Context, r repository.Repository, actor authctx.Access, userID uuid.UUID) (gen.LockMemberRow, error) {
	if userID == actor.UserID {
		return gen.LockMemberRow{}, model.ErrCannotChangeSelf
	}
	m, err := r.LockMember(ctx, gen.LockMemberParams{WorkspaceID: actor.WorkspaceID, UserID: userID})
	if repository.IsNoRows(err) {
		return m, model.ErrMemberNotFound
	}
	if err != nil {
		return m, fmt.Errorf("lock member: %w", err)
	}
	if m.IsOwner && !actor.Owner {
		return m, model.ErrOwnerProtected
	}
	return m, nil
}

// PreviewMemberRole says what moving a member to roleID would give and take away.
func (s *Service) PreviewMemberRole(ctx context.Context, workspaceID, userID, roleID uuid.UUID) (model.RoleChange, error) {
	m, err := s.GetMember(ctx, workspaceID, userID, false)
	if err != nil {
		return model.RoleChange{}, err
	}
	current, err := s.getRole(ctx, s.pool, workspaceID, m.RoleID)
	if err != nil {
		return model.RoleChange{}, err
	}
	next, err := s.getRole(ctx, s.pool, workspaceID, roleID)
	if err != nil {
		return model.RoleChange{}, err
	}
	return diffPermissions(current.Permissions, next.Permissions), nil
}

// ChangeMemberRole moves a member to another role. Extra permissions stay.
func (s *Service) ChangeMemberRole(ctx context.Context, actor authctx.Access, meta model.Meta, userID, roleID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		if m.IsOwner {
			return model.ErrOwnerRole // the owner changes only by transferring ownership
		}
		next, err := s.getRole(ctx, tx, actor.WorkspaceID, roleID)
		if err != nil {
			return err
		}
		if next.SystemKey != nil && *next.SystemKey == model.RoleOwner {
			return model.ErrOwnerRole
		}
		if next.ID == m.RoleID {
			return nil
		}
		if err := canGive(actor, next.Permissions); err != nil {
			return err
		}
		if next.UsesSeat && !m.UsesSeat {
			seats, err := s.seats(ctx, tx, actor.WorkspaceID)
			if err != nil {
				return err
			}
			if seats.Available < 1 {
				return model.ErrNotEnoughSeats
			}
		}
		current, err := s.getRole(ctx, tx, actor.WorkspaceID, m.RoleID)
		if err != nil {
			return err
		}
		if err := r.SetMemberRole(ctx, gen.SetMemberRoleParams{WorkspaceID: actor.WorkspaceID, UserID: userID, RoleID: roleID}); err != nil {
			return fmt.Errorf("set role: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: repository.ActionRoleChanged, Target: "role",
			Before: map[string]any{"role": current.Name}, After: map[string]any{"role": next.Name},
		}, meta)
	})
}

// UpdateMemberAccess changes a member's chat scope, teams or numbers.
func (s *Service) UpdateMemberAccess(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID, in model.AccessInput) error {
	if in.ChatScope != nil && *in.ChatScope != "" && !slices.Contains(model.Scopes, *in.ChatScope) {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "chat_scope", Message: "Choose which chats they can see"})
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		before, err := r.GetMember(ctx, gen.GetMemberParams{WorkspaceID: actor.WorkspaceID, UserID: userID})
		if err != nil {
			return fmt.Errorf("get member: %w", err)
		}
		after := map[string]any{}
		if in.ChatScope != nil {
			var scope *string
			if *in.ChatScope != "" {
				scope = in.ChatScope
			}
			if err := r.SetMemberScope(ctx, gen.SetMemberScopeParams{WorkspaceID: actor.WorkspaceID, UserID: userID, ChatScope: scope}); err != nil {
				return fmt.Errorf("set scope: %w", err)
			}
			after["chat_scope"] = scope
		}
		if in.SetTeams {
			ids := uniqueIDs(in.TeamIDs)
			err := r.SetMemberTeams(ctx, gen.SetMemberTeamsParams{WorkspaceID: actor.WorkspaceID, UserID: userID, TeamIds: ids})
			if repository.IsForeignKey(err) {
				return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "team_ids", Message: "Choose teams from this workspace"})
			}
			if err != nil {
				return fmt.Errorf("set teams: %w", err)
			}
			after["team_ids"] = ids
		}
		if in.SetNumber {
			ids := uniqueIDs(in.NumberIDs)
			kept, err := r.SetMemberNumbers(ctx, gen.SetMemberNumbersParams{WorkspaceID: actor.WorkspaceID, UserID: userID, NumberIds: ids})
			if err != nil {
				return fmt.Errorf("set numbers: %w", err)
			}
			if kept != int64(len(ids)) {
				return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "number_ids", Message: "Choose numbers connected to this workspace"})
			}
			after["number_ids"] = ids
		}
		if len(after) == 0 {
			return nil
		}
		if err := r.BumpAccessVersions(ctx, gen.BumpAccessVersionsParams{WorkspaceID: actor.WorkspaceID, UserIds: []uuid.UUID{userID}}); err != nil {
			return fmt.Errorf("bump access: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: repository.ActionAccessChanged, Target: "reach",
			Before: map[string]any{"chat_scope": before.ChatScope, "team_ids": before.TeamIds, "number_ids": before.NumberIds},
			After:  after,
		}, meta)
	})
}

// Suspend pauses all access without removing the member; their seat stays reserved.
func (s *Service) Suspend(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > 200 {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "reason", Message: "Keep the reason under 200 characters"})
	}
	return s.setStatus(ctx, actor, meta, userID, model.StatusSuspended, reason)
}

// Restore gives a suspended member their access back.
func (s *Service) Restore(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID) error {
	return s.setStatus(ctx, actor, meta, userID, model.StatusActive, "")
}

func (s *Service) setStatus(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID, status, reason string) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		if m.IsOwner {
			return model.ErrOwnerProtected
		}
		if m.Status == status {
			return nil
		}
		p := gen.SetMemberStatusParams{WorkspaceID: actor.WorkspaceID, UserID: userID, Status: status}
		action := repository.ActionRestored
		if status == model.StatusSuspended {
			now := s.now()
			p.SuspendedAt, p.SuspendedBy = &now, &actor.UserID
			if reason != "" {
				p.SuspendReason = &reason
			}
			action = repository.ActionSuspended
		}
		if err := r.SetMemberStatus(ctx, p); err != nil {
			return fmt.Errorf("set status: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: action, Target: "status", Before: map[string]any{"status": m.Status},
			After: map[string]any{"status": status}, Note: reason,
		}, meta)
	})
}

// Remove takes a member out of the workspace and frees their seat. Their past
// messages and notes stay in chat history.
func (s *Service) Remove(ctx context.Context, actor authctx.Access, meta model.Meta, userID uuid.UUID, in model.RemoveInput) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		m, err := lockTarget(ctx, r, actor, userID)
		if err != nil {
			return err
		}
		if m.IsOwner {
			return model.ErrOwnerProtected
		}
		if !strings.EqualFold(strings.TrimSpace(in.ConfirmName), strings.TrimSpace(m.Name)) {
			return model.ErrConfirmName
		}
		if _, err := r.DeleteMember(ctx, gen.DeleteMemberParams{WorkspaceID: actor.WorkspaceID, UserID: userID}); err != nil {
			return fmt.Errorf("remove member: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, Subject: &userID, SubjectEmail: &m.Email,
			Action: repository.ActionRemoved, Target: "membership", Before: map[string]any{"name": m.Name},
		}, meta)
	})
}

// Leave takes the signed-in member out of the workspace.
func (s *Service) Leave(ctx context.Context, me authctx.Access, meta model.Meta) error {
	if me.Owner {
		return model.ErrOwnerMustLeave
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if _, err := r.DeleteMember(ctx, gen.DeleteMemberParams{WorkspaceID: me.WorkspaceID, UserID: me.UserID}); err != nil {
			return fmt.Errorf("leave: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: me.WorkspaceID, Actor: &me.UserID, Subject: &me.UserID, Action: repository.ActionLeft, Target: "membership",
		}, meta)
	})
}

// SetAvailability records whether the member is online, away or offline.
func (s *Service) SetAvailability(ctx context.Context, me authctx.Access, availability string) error {
	if !slices.Contains(model.Availabilities, availability) {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "availability", Message: "Choose online, away or offline"})
	}
	if err := repository.NewRepository(s.pool).SetAvailability(ctx, gen.SetAvailabilityParams{
		WorkspaceID: me.WorkspaceID, UserID: me.UserID, Availability: availability,
	}); err != nil {
		return fmt.Errorf("set availability: %w", err)
	}
	return nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
