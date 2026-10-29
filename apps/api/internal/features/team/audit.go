package team

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
)

// Access log actions.
const (
	ActionRoleChanged       = "role_changed"
	ActionPermissionGranted = "permission_granted"
	ActionPermissionDenied  = "permission_denied"
	ActionPermissionRevoked = "permission_revoked"
	ActionPermissionExpired = "permission_expired"
	ActionInvited           = "invited"
	ActionInviteResent      = "invite_resent"
	ActionInviteRevoked     = "invite_revoked"
	ActionInviteAccepted    = "invite_accepted"
	ActionRemoved           = "removed"
	ActionLeft              = "left"
	ActionSuspended         = "suspended"
	ActionRestored          = "restored"
	ActionAccessChanged     = "access_changed"
	ActionTeamChanged       = "team_changed"
	ActionRoleCreated       = "role_created"
	ActionRoleUpdated       = "role_updated"
	ActionRoleDeleted       = "role_deleted"
)

// event is one access log entry, written in the same transaction as the change
// it records, so a change without an entry can't commit.
type event struct {
	workspaceID  uuid.UUID
	actor        *uuid.UUID
	subject      *uuid.UUID
	subjectEmail *string
	action       string
	target       string
	before       any
	after        any
	note         string
}

func (r repository) log(ctx context.Context, e event, meta Meta) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("log id: %w", err)
	}
	before, err := jsonOrNil(e.before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.after)
	if err != nil {
		return err
	}
	var ip *netip.Addr
	if a, err := netip.ParseAddr(meta.IP); err == nil {
		ip = &a
	}
	if err := r.InsertAccessLog(ctx, gen.InsertAccessLogParams{
		ID: id, WorkspaceID: e.workspaceID, ActorUserID: e.actor, SubjectUserID: e.subject, SubjectEmail: e.subjectEmail,
		Action: e.action, Target: e.target, BeforeValue: before, AfterValue: after, Note: e.note,
		Ip: ip, UserAgent: truncate(meta.UserAgent, 300),
	}); err != nil {
		return fmt.Errorf("write access log: %w", err)
	}
	return nil
}

func jsonOrNil(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("log value: %w", err)
	}
	return b, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
