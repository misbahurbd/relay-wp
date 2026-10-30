package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/mailer"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const (
	maxInvitesPerRequest = 50
	maxSends             = 5
)

// ListInvitations returns pending invites and those that ended in the last 30 days.
func (s *Service) ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]model.Invitation, error) {
	rows, err := repository.NewRepository(s.pool).ListInvitations(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	now := s.now()
	out := make([]model.Invitation, len(rows))
	for i, r := range rows {
		out[i] = model.Invitation{
			ID: r.ID, Email: r.Email, RoleID: r.RoleID, RoleName: r.RoleName, ExpiresAt: r.ExpiresAt, LastSentAt: r.LastSentAt,
			SendCount: r.SendCount, RenewalRequestedAt: r.RenewalRequestedAt, CreatedAt: r.CreatedAt,
			InvitedByName: r.InvitedByName, TeamIDs: r.TeamIds,
		}
		out[i].Status = inviteState(r.Status, r.ExpiresAt, now)
	}
	return out, nil
}

func inviteState(status string, expiresAt, now time.Time) string {
	if status == model.InvitePending && !expiresAt.After(now) {
		return model.InviteExpired
	}
	return status
}

// Invite sends one invitation per email, all with the same role and reach.
func (s *Service) Invite(ctx context.Context, actor authctx.Access, meta model.Meta, in model.InviteInput) ([]model.Invitation, error) {
	emails, err := s.validateInvite(&in)
	if err != nil {
		return nil, err
	}
	ttl := model.InviteTTLs[in.ExpiresIn]
	role, err := s.getRole(ctx, s.pool, actor.WorkspaceID, in.RoleID)
	if apperr.IsKind(err, apperr.KindNotFound) {
		return nil, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "role_id", Message: "Choose a role"})
	}
	if err != nil {
		return nil, err
	}
	if role.SystemKey != nil && *role.SystemKey == model.RoleOwner {
		return nil, model.ErrOwnerRole
	}
	gives := role.Permissions
	for _, o := range in.Overrides {
		if o.Effect == model.EffectGrant {
			gives = append(gives, o.PermissionKey)
		}
	}
	if err := canGive(actor, gives); err != nil {
		return nil, err
	}
	inviter, err := s.inviterName(ctx, actor)
	if err != nil {
		return nil, err
	}
	planLimit := -1
	if role.UsesSeat {
		if planLimit, err = s.memberLimit(ctx, actor.WorkspaceID); err != nil {
			return nil, err
		}
	}

	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if role.UsesSeat {
			seats, err := s.seats(ctx, tx, actor.WorkspaceID)
			if err != nil {
				return err
			}
			if err := overMemberLimit(planLimit, seats.Used+seats.Reserved, int64(len(emails))); err != nil {
				return err
			}
			if seats.Limit >= 0 && seats.Available < int64(len(emails)) {
				return model.ErrNotEnoughSeats.WithDetails(apperr.Detail{
					Field:   "emails",
					Message: fmt.Sprintf("%d seats needed, %d available", len(emails), seats.Available),
				})
			}
		}
		if n, err := r.CountWorkspaceNumbers(ctx, gen.CountWorkspaceNumbersParams{
			WorkspaceID: actor.WorkspaceID, NumberIds: in.NumberIDs,
		}); err != nil {
			return fmt.Errorf("check numbers: %w", err)
		} else if n != int64(len(in.NumberIDs)) {
			return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "number_ids", Message: "Choose numbers connected to this workspace"})
		}
		for _, email := range emails {
			if err := s.createInvite(ctx, tx, actor, meta, in, role, inviter, email, ttl); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	all, err := s.ListInvitations(ctx, actor.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]model.Invitation, 0, len(emails))
	for _, inv := range all {
		if inv.Status == model.InvitePending && slices.Contains(emails, strings.ToLower(inv.Email)) {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (s *Service) createInvite(ctx context.Context, tx pgx.Tx, actor authctx.Access, meta model.Meta, in model.InviteInput,
	role model.Role, inviter, email string, ttl time.Duration,
) error {
	r := repository.NewRepository(tx)
	member, err := r.IsMemberEmail(ctx, gen.IsMemberEmailParams{WorkspaceID: actor.WorkspaceID, Email: email})
	if err != nil {
		return fmt.Errorf("check member: %w", err)
	}
	if member {
		return model.ErrAlreadyMember.WithDetails(apperr.Detail{Field: "emails", Message: email})
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("invite id: %w", err)
	}
	raw, hash, err := newToken()
	if err != nil {
		return err
	}
	expires := s.now().Add(ttl)
	err = r.CreateInvitation(ctx, gen.CreateInvitationParams{
		ID: id, WorkspaceID: actor.WorkspaceID, Email: email, RoleID: role.ID, ChatScope: in.ChatScope,
		TokenHash: hash, PersonalMessage: strings.TrimSpace(in.PersonalMessage), Require2fa: in.Require2FA,
		InvitedBy: &actor.UserID, ExpiresAt: expires,
	})
	if repository.IsUnique(err, "invitations_pending_email_idx") {
		return model.ErrInvitePending.WithDetails(apperr.Detail{Field: "emails", Message: email})
	}
	if err != nil {
		return fmt.Errorf("create invite: %w", err)
	}
	if len(in.TeamIDs) > 0 {
		err := r.AddInvitationTeams(ctx, gen.AddInvitationTeamsParams{WorkspaceID: actor.WorkspaceID, InvitationID: id, TeamIds: in.TeamIDs})
		if repository.IsForeignKey(err) {
			return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "team_ids", Message: "Choose teams from this workspace"})
		}
		if err != nil {
			return fmt.Errorf("invite teams: %w", err)
		}
	}
	if len(in.NumberIDs) > 0 {
		if err := r.AddInvitationNumbers(ctx, gen.AddInvitationNumbersParams{WorkspaceID: actor.WorkspaceID, InvitationID: id, NumberIds: in.NumberIDs}); err != nil {
			return fmt.Errorf("invite numbers: %w", err)
		}
	}
	if len(in.Overrides) > 0 {
		keys, effects := make([]string, len(in.Overrides)), make([]string, len(in.Overrides))
		for i, o := range in.Overrides {
			keys[i], effects[i] = o.PermissionKey, o.Effect
		}
		if err := r.AddInvitationOverrides(ctx, gen.AddInvitationOverridesParams{
			WorkspaceID: actor.WorkspaceID, InvitationID: id, Keys: keys, Effects: effects,
		}); err != nil {
			return fmt.Errorf("invite overrides: %w", err)
		}
	}
	if err := s.enqueueEmail(ctx, tx, inviteEmail(email, inviter, s.workspaceName(ctx, tx, actor.WorkspaceID), role.Name,
		in.PersonalMessage, s.inviteLink(raw), expires)); err != nil {
		return err
	}
	return r.Log(ctx, repository.Event{
		WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, SubjectEmail: &email, Action: repository.ActionInvited,
		Target: id.String(), After: map[string]any{"role": role.Name, "expires_at": expires},
	}, meta)
}

func (s *Service) validateInvite(in *model.InviteInput) ([]string, error) {
	var details []apperr.Detail
	var emails []string
	for _, raw := range in.Emails {
		e := strings.ToLower(strings.TrimSpace(raw))
		if e == "" {
			continue
		}
		addr, err := mail.ParseAddress(e)
		if err != nil || addr.Address != e || !strings.Contains(e[strings.LastIndex(e, "@")+1:], ".") {
			details = append(details, apperr.Detail{Field: "emails", Message: "Check this address: " + raw})
			continue
		}
		if !slices.Contains(emails, e) {
			emails = append(emails, e)
		}
	}
	if len(emails) == 0 && len(details) == 0 {
		details = append(details, apperr.Detail{Field: "emails", Message: "Add at least one email address"})
	}
	if len(emails) > maxInvitesPerRequest {
		details = append(details, apperr.Detail{Field: "emails", Message: fmt.Sprintf("Invite up to %d people at a time", maxInvitesPerRequest)})
	}
	if in.ExpiresIn == "" {
		in.ExpiresIn = "7d"
	}
	if _, ok := model.InviteTTLs[in.ExpiresIn]; !ok {
		details = append(details, apperr.Detail{Field: "expires_in", Message: "Choose 24 hours, 7 days or 30 days"})
	}
	if in.ChatScope != nil && !slices.Contains(model.Scopes, *in.ChatScope) {
		details = append(details, apperr.Detail{Field: "chat_scope", Message: "Choose which chats they can open"})
	}
	if utf8.RuneCountInString(strings.TrimSpace(in.PersonalMessage)) > 500 {
		details = append(details, apperr.Detail{Field: "personal_message", Message: "Keep the message under 500 characters"})
	}
	for _, o := range in.Overrides {
		if _, ok := authctx.PermissionByKey(o.PermissionKey); !ok || (o.Effect != model.EffectGrant && o.Effect != model.EffectDeny) {
			details = append(details, apperr.Detail{Field: "overrides", Message: "Unknown permission " + o.PermissionKey})
		}
	}
	in.TeamIDs, in.NumberIDs = uniqueIDs(in.TeamIDs), uniqueIDs(in.NumberIDs)
	if len(details) > 0 {
		return nil, model.ErrInvalidInput.WithDetails(details...)
	}
	return emails, nil
}

// ResendInvitation sends a new link with a new expiry; the old link stops working.
func (s *Service) ResendInvitation(ctx context.Context, actor authctx.Access, meta model.Meta, invitationID uuid.UUID) error {
	inviter, err := s.inviterName(ctx, actor)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		inv, err := r.LockInvitation(ctx, gen.LockInvitationParams{WorkspaceID: actor.WorkspaceID, InvitationID: invitationID})
		if repository.IsNoRows(err) {
			return model.ErrInviteNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invite: %w", err)
		}
		switch inv.Status {
		case model.InviteAccepted:
			return model.ErrInviteUsed
		case model.InviteRevoked:
			return model.ErrInviteRevoked
		}
		if inv.SendCount >= maxSends {
			return model.ErrResendLimit
		}
		role, err := s.getRole(ctx, tx, actor.WorkspaceID, inv.RoleID)
		if err != nil {
			return err
		}
		raw, hash, err := newToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(model.InviteTTLs["7d"])
		if err := r.ResendInvitation(ctx, gen.ResendInvitationParams{
			WorkspaceID: actor.WorkspaceID, InvitationID: invitationID, TokenHash: hash, ExpiresAt: expires,
		}); err != nil {
			return fmt.Errorf("resend invite: %w", err)
		}
		if err := s.enqueueEmail(ctx, tx, inviteEmail(inv.Email, inviter, s.workspaceName(ctx, tx, actor.WorkspaceID),
			role.Name, "", s.inviteLink(raw), expires)); err != nil {
			return err
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, SubjectEmail: &inv.Email, Action: repository.ActionInviteResent,
			Target: invitationID.String(), After: map[string]any{"expires_at": expires},
		}, meta)
	})
}

// RevokeInvitation cancels a pending invite; its link stops working at once and the seat is freed.
func (s *Service) RevokeInvitation(ctx context.Context, actor authctx.Access, meta model.Meta, invitationID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		inv, err := r.LockInvitation(ctx, gen.LockInvitationParams{WorkspaceID: actor.WorkspaceID, InvitationID: invitationID})
		if repository.IsNoRows(err) {
			return model.ErrInviteNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invite: %w", err)
		}
		if inv.Status == model.InviteAccepted {
			return model.ErrInviteUsed
		}
		if inv.Status == model.InviteRevoked {
			return nil
		}
		if err := r.RevokeInvitation(ctx, gen.RevokeInvitationParams{
			WorkspaceID: actor.WorkspaceID, InvitationID: invitationID, RevokedBy: &actor.UserID,
		}); err != nil {
			return fmt.Errorf("revoke invite: %w", err)
		}
		return r.Log(ctx, repository.Event{
			WorkspaceID: actor.WorkspaceID, Actor: &actor.UserID, SubjectEmail: &inv.Email, Action: repository.ActionInviteRevoked,
			Target: invitationID.String(),
		}, meta)
	})
}

func (s *Service) inviteLink(raw string) string { return s.cfg.AppBaseURL + "/invite#t=" + raw }

func (s *Service) inviterName(ctx context.Context, actor authctx.Access) (string, error) {
	m, err := repository.NewRepository(s.pool).GetMember(ctx, gen.GetMemberParams{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID})
	if err != nil {
		return "", fmt.Errorf("inviter: %w", err)
	}
	return m.Name, nil
}

func (s *Service) workspaceName(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) string {
	name, err := repository.NewRepository(db).GetWorkspaceName(ctx, workspaceID)
	if err != nil {
		return "your team"
	}
	return name
}

func (s *Service) enqueueEmail(ctx context.Context, tx pgx.Tx, msg mailer.Message) error {
	if _, err := s.queue.InsertTx(ctx, tx, mailer.SendArgs{Message: msg}, nil); err != nil {
		return fmt.Errorf("queue email: %w", err)
	}
	return nil
}

// newToken returns a random 256-bit invite token and its SHA-256. Only the hash is stored.
func newToken() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}
