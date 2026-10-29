package team

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
func (s *Service) ListInvitations(ctx context.Context, workspaceID uuid.UUID) ([]Invitation, error) {
	rows, err := newRepository(s.pool).ListInvitations(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	now := s.now()
	out := make([]Invitation, len(rows))
	for i, r := range rows {
		out[i] = Invitation{
			ID: r.ID, Email: r.Email, RoleID: r.RoleID, RoleName: r.RoleName, ExpiresAt: r.ExpiresAt, LastSentAt: r.LastSentAt,
			SendCount: r.SendCount, RenewalRequestedAt: r.RenewalRequestedAt, CreatedAt: r.CreatedAt,
			InvitedByName: r.InvitedByName, TeamIDs: r.TeamIds,
		}
		out[i].Status = inviteState(r.Status, r.ExpiresAt, now)
	}
	return out, nil
}

func inviteState(status string, expiresAt, now time.Time) string {
	if status == InvitePending && !expiresAt.After(now) {
		return InviteExpired
	}
	return status
}

// Invite sends one invitation per email, all with the same role and reach.
func (s *Service) Invite(ctx context.Context, actor authctx.Access, meta Meta, in InviteInput) ([]Invitation, error) {
	emails, err := s.validateInvite(&in)
	if err != nil {
		return nil, err
	}
	ttl := inviteTTLs[in.ExpiresIn]
	role, err := s.getRole(ctx, s.pool, actor.WorkspaceID, in.RoleID)
	if apperr.IsKind(err, apperr.KindNotFound) {
		return nil, ErrInvalidInput.WithDetails(apperr.Detail{Field: "role_id", Message: "Choose a role"})
	}
	if err != nil {
		return nil, err
	}
	if role.SystemKey != nil && *role.SystemKey == RoleOwner {
		return nil, ErrOwnerRole
	}
	gives := role.Permissions
	for _, o := range in.Overrides {
		if o.Effect == EffectGrant {
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

	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		if role.UsesSeat {
			seats, err := s.seats(ctx, tx, actor.WorkspaceID)
			if err != nil {
				return err
			}
			if seats.Available < int64(len(emails)) {
				return ErrNotEnoughSeats.WithDetails(apperr.Detail{
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
			return ErrInvalidInput.WithDetails(apperr.Detail{Field: "number_ids", Message: "Choose numbers connected to this workspace"})
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
	out := make([]Invitation, 0, len(emails))
	for _, inv := range all {
		if inv.Status == InvitePending && slices.Contains(emails, strings.ToLower(inv.Email)) {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (s *Service) createInvite(ctx context.Context, tx pgx.Tx, actor authctx.Access, meta Meta, in InviteInput,
	role Role, inviter, email string, ttl time.Duration,
) error {
	r := newRepository(tx)
	member, err := r.IsMemberEmail(ctx, gen.IsMemberEmailParams{WorkspaceID: actor.WorkspaceID, Email: email})
	if err != nil {
		return fmt.Errorf("check member: %w", err)
	}
	if member {
		return ErrAlreadyMember.WithDetails(apperr.Detail{Field: "emails", Message: email})
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
	if isUnique(err, "invitations_pending_email_idx") {
		return ErrInvitePending.WithDetails(apperr.Detail{Field: "emails", Message: email})
	}
	if err != nil {
		return fmt.Errorf("create invite: %w", err)
	}
	if len(in.TeamIDs) > 0 {
		err := r.AddInvitationTeams(ctx, gen.AddInvitationTeamsParams{WorkspaceID: actor.WorkspaceID, InvitationID: id, TeamIds: in.TeamIDs})
		if isForeignKey(err) {
			return ErrInvalidInput.WithDetails(apperr.Detail{Field: "team_ids", Message: "Choose teams from this workspace"})
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
	return r.log(ctx, event{
		workspaceID: actor.WorkspaceID, actor: &actor.UserID, subjectEmail: &email, action: ActionInvited,
		target: id.String(), after: map[string]any{"role": role.Name, "expires_at": expires},
	}, meta)
}

func (s *Service) validateInvite(in *InviteInput) ([]string, error) {
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
	if _, ok := inviteTTLs[in.ExpiresIn]; !ok {
		details = append(details, apperr.Detail{Field: "expires_in", Message: "Choose 24 hours, 7 days or 30 days"})
	}
	if in.ChatScope != nil && !slices.Contains(scopes, *in.ChatScope) {
		details = append(details, apperr.Detail{Field: "chat_scope", Message: "Choose which chats they can open"})
	}
	if utf8.RuneCountInString(strings.TrimSpace(in.PersonalMessage)) > 500 {
		details = append(details, apperr.Detail{Field: "personal_message", Message: "Keep the message under 500 characters"})
	}
	for _, o := range in.Overrides {
		if _, ok := authctx.PermissionByKey(o.PermissionKey); !ok || (o.Effect != EffectGrant && o.Effect != EffectDeny) {
			details = append(details, apperr.Detail{Field: "overrides", Message: "Unknown permission " + o.PermissionKey})
		}
	}
	in.TeamIDs, in.NumberIDs = uniqueIDs(in.TeamIDs), uniqueIDs(in.NumberIDs)
	if len(details) > 0 {
		return nil, ErrInvalidInput.WithDetails(details...)
	}
	return emails, nil
}

// ResendInvitation sends a new link with a new expiry; the old link stops working.
func (s *Service) ResendInvitation(ctx context.Context, actor authctx.Access, meta Meta, invitationID uuid.UUID) error {
	inviter, err := s.inviterName(ctx, actor)
	if err != nil {
		return err
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		inv, err := r.LockInvitation(ctx, gen.LockInvitationParams{WorkspaceID: actor.WorkspaceID, InvitationID: invitationID})
		if isNoRows(err) {
			return ErrInviteNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invite: %w", err)
		}
		switch inv.Status {
		case InviteAccepted:
			return ErrInviteUsed
		case InviteRevoked:
			return ErrInviteRevoked
		}
		if inv.SendCount >= maxSends {
			return ErrResendLimit
		}
		role, err := s.getRole(ctx, tx, actor.WorkspaceID, inv.RoleID)
		if err != nil {
			return err
		}
		raw, hash, err := newToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(inviteTTLs["7d"])
		if err := r.ResendInvitation(ctx, gen.ResendInvitationParams{
			WorkspaceID: actor.WorkspaceID, InvitationID: invitationID, TokenHash: hash, ExpiresAt: expires,
		}); err != nil {
			return fmt.Errorf("resend invite: %w", err)
		}
		if err := s.enqueueEmail(ctx, tx, inviteEmail(inv.Email, inviter, s.workspaceName(ctx, tx, actor.WorkspaceID),
			role.Name, "", s.inviteLink(raw), expires)); err != nil {
			return err
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, subjectEmail: &inv.Email, action: ActionInviteResent,
			target: invitationID.String(), after: map[string]any{"expires_at": expires},
		}, meta)
	})
}

// RevokeInvitation cancels a pending invite; its link stops working at once and the seat is freed.
func (s *Service) RevokeInvitation(ctx context.Context, actor authctx.Access, meta Meta, invitationID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		inv, err := r.LockInvitation(ctx, gen.LockInvitationParams{WorkspaceID: actor.WorkspaceID, InvitationID: invitationID})
		if isNoRows(err) {
			return ErrInviteNotFound
		}
		if err != nil {
			return fmt.Errorf("lock invite: %w", err)
		}
		if inv.Status == InviteAccepted {
			return ErrInviteUsed
		}
		if inv.Status == InviteRevoked {
			return nil
		}
		if err := r.RevokeInvitation(ctx, gen.RevokeInvitationParams{
			WorkspaceID: actor.WorkspaceID, InvitationID: invitationID, RevokedBy: &actor.UserID,
		}); err != nil {
			return fmt.Errorf("revoke invite: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, subjectEmail: &inv.Email, action: ActionInviteRevoked,
			target: invitationID.String(),
		}, meta)
	})
}

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

func (s *Service) inviteLink(raw string) string { return s.cfg.AppBaseURL + "/invite#t=" + raw }

func (s *Service) inviterName(ctx context.Context, actor authctx.Access) (string, error) {
	m, err := newRepository(s.pool).GetMember(ctx, gen.GetMemberParams{WorkspaceID: actor.WorkspaceID, UserID: actor.UserID})
	if err != nil {
		return "", fmt.Errorf("inviter: %w", err)
	}
	return m.Name, nil
}

func (s *Service) workspaceName(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) string {
	name, err := newRepository(db).GetWorkspaceName(ctx, workspaceID)
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
