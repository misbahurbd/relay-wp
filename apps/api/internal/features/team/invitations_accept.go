package team

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// PreviewInvitation is what the invite link shows before joining. It needs no
// sign-in; unknown tokens get the same not-found error.
func (s *Service) PreviewInvitation(ctx context.Context, rawToken string) (InvitePreview, error) {
	r := newRepository(s.pool)
	inv, err := r.GetInvitationByTokenHash(ctx, hashToken(rawToken))
	if isNoRows(err) {
		return InvitePreview{}, ErrInviteNotFound
	}
	if err != nil {
		return InvitePreview{}, fmt.Errorf("find invite: %w", err)
	}
	p := InvitePreview{
		State: inviteState(inv.Status, inv.ExpiresAt, s.now()), WorkspaceName: inv.WorkspaceName, Email: inv.Email,
		RoleName: inv.RoleName, InvitedByName: inv.InvitedByName, InvitedByRole: inv.InvitedByRole,
		ExpiresAt: inv.ExpiresAt, Message: inv.PersonalMessage, WorkspaceID: inv.WorkspaceID,
	}
	if p.State == InvitePending {
		p.State = "valid"
		bits, err := s.invitedBits(ctx, r, inv.ID, inv.RoleID, inv.SystemKey)
		if err != nil {
			return InvitePreview{}, err
		}
		p.PermissionKeys = bits.Keys()
		ids := gen.InvitationTeamNamesParams{WorkspaceID: inv.WorkspaceID, InvitationID: inv.ID}
		if p.TeamNames, err = r.InvitationTeamNames(ctx, ids); err != nil {
			return InvitePreview{}, fmt.Errorf("invite teams: %w", err)
		}
		nums, err := r.InvitationNumberLabels(ctx, gen.InvitationNumberLabelsParams(ids))
		if err != nil {
			return InvitePreview{}, fmt.Errorf("invite numbers: %w", err)
		}
		for _, n := range nums {
			p.NumberLabels = append(p.NumberLabels, n.VerifiedName+" ("+n.DisplayPhoneNumber+")")
		}
		labels, err := s.Permissions(ctx)
		if err != nil {
			return InvitePreview{}, err
		}
		for _, l := range labels {
			if slices.Contains(p.PermissionKeys, l.Key) {
				p.CanDo = append(p.CanDo, l.Label)
			}
		}
	}
	return p, nil
}

// invitedBits is what an invitee would be able to do: the role plus the invite's overrides.
func (s *Service) invitedBits(ctx context.Context, r repository, invitationID, roleID uuid.UUID, systemKey *string) (authctx.Bits, error) {
	if systemKey != nil && *systemKey == RoleOwner {
		return authctx.AllBits, nil
	}
	b, err := r.RoleBits(ctx, roleID)
	if err != nil {
		return 0, fmt.Errorf("role bits: %w", err)
	}
	bits := authctx.Bits(b) //nolint:gosec // bits are 0–62, never negative
	overrides, err := r.InvitationOverrides(ctx, invitationID)
	if err != nil {
		return 0, fmt.Errorf("invite overrides: %w", err)
	}
	for _, o := range overrides {
		p, ok := authctx.PermissionByKey(o.PermissionKey)
		if !ok {
			continue
		}
		if o.Effect == EffectGrant {
			bits = bits.With(p)
		} else {
			bits &^= 1 << p
		}
	}
	return bits, nil
}

// AcceptInvitation adds the signed-in user to the invite's workspace with its
// role, teams, numbers and permissions. It returns the workspace ID.
func (s *Service) AcceptInvitation(ctx context.Context, rawToken string, userID uuid.UUID, meta Meta) (uuid.UUID, error) {
	email, verified, _, err := s.accounts.AccountInfo(ctx, userID)
	if err != nil {
		return uuid.Nil, err
	}
	inv, err := newRepository(s.pool).GetInvitationByTokenHash(ctx, hashToken(rawToken))
	if isNoRows(err) {
		return uuid.Nil, ErrInviteNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("find invite: %w", err)
	}
	if !strings.EqualFold(email, inv.Email) {
		return uuid.Nil, ErrInviteEmail
	}
	if !verified {
		return uuid.Nil, ErrEmailNotVerified
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		locked, err := r.LockInvitation(ctx, gen.LockInvitationParams{WorkspaceID: inv.WorkspaceID, InvitationID: inv.ID})
		if err != nil {
			return fmt.Errorf("lock invite: %w", err)
		}
		switch inviteState(locked.Status, locked.ExpiresAt, s.now()) {
		case InviteAccepted:
			return ErrInviteUsed
		case InviteRevoked:
			return ErrInviteRevoked
		case InviteExpired:
			return ErrInviteExpired
		}
		if _, err := r.LockMember(ctx, gen.LockMemberParams{WorkspaceID: inv.WorkspaceID, UserID: userID}); err == nil {
			return ErrAlreadyMember
		} else if !isNoRows(err) {
			return fmt.Errorf("check member: %w", err)
		}
		if inv.UsesSeat {
			// This invite's seat is already counted as reserved, so it fits unless the plan shrank.
			seats, err := s.seats(ctx, tx, inv.WorkspaceID)
			if err != nil {
				return err
			}
			if seats.Used+seats.Reserved > int64(seats.Limit) {
				return ErrNotEnoughSeats
			}
		}
		return s.join(ctx, tx, inv, userID, email, meta)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return inv.WorkspaceID, nil
}

func (s *Service) join(ctx context.Context, tx pgx.Tx, inv gen.GetInvitationByTokenHashRow, userID uuid.UUID, email string, meta Meta) error {
	r := newRepository(tx)
	if err := r.AddMember(ctx, gen.AddMemberParams{
		WorkspaceID: inv.WorkspaceID, UserID: userID, RoleID: inv.RoleID, ChatScope: inv.ChatScope, InvitedBy: inv.InvitedBy,
	}); err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	teams, err := r.InvitationTeamIDs(ctx, inv.ID)
	if err != nil {
		return fmt.Errorf("invite teams: %w", err)
	}
	if err := r.SetMemberTeams(ctx, gen.SetMemberTeamsParams{WorkspaceID: inv.WorkspaceID, UserID: userID, TeamIds: teams}); err != nil {
		return fmt.Errorf("join teams: %w", err)
	}
	numbers, err := r.InvitationNumberIDs(ctx, inv.ID)
	if err != nil {
		return fmt.Errorf("invite numbers: %w", err)
	}
	if _, err := r.SetMemberNumbers(ctx, gen.SetMemberNumbersParams{WorkspaceID: inv.WorkspaceID, UserID: userID, NumberIds: numbers}); err != nil {
		return fmt.Errorf("join numbers: %w", err)
	}
	overrides, err := r.InvitationOverrides(ctx, inv.ID)
	if err != nil {
		return fmt.Errorf("invite overrides: %w", err)
	}
	for _, o := range overrides {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("override id: %w", err)
		}
		if err := r.CreateOverride(ctx, gen.CreateOverrideParams{
			ID: id, WorkspaceID: inv.WorkspaceID, UserID: userID, PermissionKey: o.PermissionKey, Effect: o.Effect,
			Reason: "Set on the invite", GrantedBy: inv.InvitedBy,
		}); err != nil {
			return fmt.Errorf("invite override: %w", err)
		}
	}
	if err := r.AcceptInvitation(ctx, gen.AcceptInvitationParams{InvitationID: inv.ID, UserID: &userID}); err != nil {
		return fmt.Errorf("accept invite: %w", err)
	}
	if inv.InvitedBy != nil {
		if to, _, _, err := s.accounts.AccountInfo(ctx, *inv.InvitedBy); err == nil {
			if err := s.enqueueEmail(ctx, tx, joinedEmail(to, email, inv.WorkspaceName, inv.RoleName,
				s.cfg.AppBaseURL+"/w/"+inv.WorkspaceID.String()+"/team")); err != nil {
				return err
			}
		}
	}
	return r.log(ctx, event{
		workspaceID: inv.WorkspaceID, actor: &userID, subject: &userID, subjectEmail: &email,
		action: ActionInviteAccepted, target: inv.ID.String(), after: map[string]any{"role": inv.RoleName},
	}, meta)
}

// RequestRenewal lets someone with an expired link ask the inviter for a new one, once.
func (s *Service) RequestRenewal(ctx context.Context, rawToken string) error {
	r := newRepository(s.pool)
	inv, err := r.GetInvitationByTokenHash(ctx, hashToken(rawToken))
	if isNoRows(err) {
		return ErrInviteNotFound
	}
	if err != nil {
		return fmt.Errorf("find invite: %w", err)
	}
	if inviteState(inv.Status, inv.ExpiresAt, s.now()) != InviteExpired {
		return nil
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		n, err := newRepository(tx).RequestInvitationRenewal(ctx, inv.ID)
		if err != nil {
			return fmt.Errorf("request renewal: %w", err)
		}
		if n == 0 || inv.InvitedBy == nil {
			return nil
		}
		to, _, _, err := s.accounts.AccountInfo(ctx, *inv.InvitedBy)
		if err != nil {
			return err
		}
		return s.enqueueEmail(ctx, tx, renewalEmail(to, inv.Email, inv.WorkspaceName,
			s.cfg.AppBaseURL+"/w/"+inv.WorkspaceID.String()+"/team"))
	})
}

// InviteEmail returns the address a usable invite link was sent to. Implements auth.Invites.
func (s *Service) InviteEmail(ctx context.Context, rawToken string) (string, error) {
	inv, err := newRepository(s.pool).GetInvitationByTokenHash(ctx, hashToken(rawToken))
	if isNoRows(err) {
		return "", ErrInviteNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find invite: %w", err)
	}
	switch inviteState(inv.Status, inv.ExpiresAt, s.now()) {
	case InviteExpired:
		return "", ErrInviteExpired
	case InviteRevoked:
		return "", ErrInviteRevoked
	case InviteAccepted:
		return "", ErrInviteUsed
	}
	return inv.Email, nil
}

// JoinByInvite accepts an invite for a just-created account. Implements auth.Invites.
func (s *Service) JoinByInvite(ctx context.Context, rawToken string, userID uuid.UUID, ip, userAgent string) (uuid.UUID, error) {
	return s.AcceptInvitation(ctx, rawToken, userID, Meta{IP: ip, UserAgent: userAgent})
}
