package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"

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

// Event is one access log entry, written in the same transaction as the change
// it records, so a change without an entry can't commit.
type Event struct {
	WorkspaceID  uuid.UUID
	Actor        *uuid.UUID
	Subject      *uuid.UUID
	SubjectEmail *string
	Action       string
	Target       string
	Before       any
	After        any
	Note         string
}

// Log writes one access log entry in the caller's transaction.
func (r Repository) Log(ctx context.Context, e Event, meta model.Meta) error {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("log id: %w", err)
	}
	before, err := jsonOrNil(e.Before)
	if err != nil {
		return err
	}
	after, err := jsonOrNil(e.After)
	if err != nil {
		return err
	}
	var ip *netip.Addr
	if a, err := netip.ParseAddr(meta.IP); err == nil {
		ip = &a
	}
	if err := r.InsertAccessLog(ctx, gen.InsertAccessLogParams{
		ID: id, WorkspaceID: e.WorkspaceID, ActorUserID: e.Actor, SubjectUserID: e.Subject, SubjectEmail: e.SubjectEmail,
		Action: e.Action, Target: e.Target, BeforeValue: before, AfterValue: after, Note: e.Note,
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
