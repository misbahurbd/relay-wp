package team

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// System role keys. Every workspace has one role of each.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleAgent  = "agent"
	RoleViewer = "viewer"
)

// Member statuses.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

// Override effects.
const (
	EffectGrant = "grant"
	EffectDeny  = "deny"
)

// Invitation statuses as shown to people; "expired" is pending past its end date.
const (
	InvitePending  = "pending"
	InviteAccepted = "accepted"
	InviteRevoked  = "revoked"
	InviteExpired  = "expired"
)

var (
	colors         = []string{"green", "blue", "amber", "red", "gray"}
	scopes         = []string{authctx.ScopeAssigned, authctx.ScopeTeam, authctx.ScopeAll}
	routingMethods = []string{"round_robin", "least_busy", "manual"}
	availabilities = []string{"online", "away", "offline"}
	inviteTTLs     = map[string]time.Duration{"24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}
)

// systemRole is one of the four roles every workspace starts with.
type systemRole struct {
	key, name, description, color, scope string
	perms                                []authctx.Permission
}

// systemRoles are the defaults from docs/PHASE-3-TEAM-ACCESS.md section 3.
// "Reset to default" restores them. Migration 00010 backfills the same set.
var systemRoles = []systemRole{
	{
		RoleOwner, "Owner", "Full access, including billing and deleting the workspace", "green", authctx.ScopeAll,
		authctx.AllPermissions(),
	},
	{
		RoleAdmin, "Admin", "Manages everything except paying and changing the plan", "blue", authctx.ScopeAll,
		allExcept(authctx.PermBillingPay),
	},
	{RoleAgent, "Agent", "Answers customers in the inbox", "amber", authctx.ScopeTeam, []authctx.Permission{
		authctx.PermInboxReply, authctx.PermInboxSendTemplate, authctx.PermInboxCloseReassign, authctx.PermInboxNotes,
		authctx.PermContactsView, authctx.PermContactsEdit, authctx.PermBroadcastsReports,
	}},
	{RoleViewer, "Viewer", "Read-only access to chats and reports", "gray", authctx.ScopeTeam, []authctx.Permission{
		authctx.PermContactsView, authctx.PermBroadcastsReports,
	}},
}

func allExcept(skip authctx.Permission) []authctx.Permission {
	var out []authctx.Permission
	for _, p := range authctx.AllPermissions() {
		if p != skip {
			out = append(out, p)
		}
	}
	return out
}

func systemRoleDefaults(key string) (systemRole, bool) {
	for _, r := range systemRoles {
		if r.key == key {
			return r, true
		}
	}
	return systemRole{}, false
}

func keysOf(perms []authctx.Permission) []string {
	out := make([]string, len(perms))
	for i, p := range perms {
		out[i] = p.Key()
	}
	return out
}

// Permission is one entry of the catalog, as the Roles editor shows it.
type Permission struct {
	Key       string
	Group     string
	Label     string
	Sensitive bool
}

// Role is a named set of permissions plus a default chat scope.
type Role struct {
	ID          uuid.UUID
	SystemKey   *string
	Name        string
	Description string
	Color       string
	ChatScope   string
	UsesSeat    bool
	Locked      bool
	Version     int32
	Permissions []string
	MemberCount int64
}

// RoleInput is a create or edit from the Roles editor.
type RoleInput struct {
	Name        string
	Description string
	Color       string
	ChatScope   string
	Permissions []string
	// CopyFrom starts a new role from another role's permissions and scope.
	CopyFrom *uuid.UUID
	// ExpectedVersion guards an edit against a change saved in the meantime.
	ExpectedVersion int32
	NotifyMembers   bool
}

// RoleChange is what a role edit would give and take away, shown before saving.
type RoleChange struct {
	Gains         []string
	Losses        []string
	SensitiveGain bool
	Affected      []MemberRef
}

// MemberRef names a member in a preview or log.
type MemberRef struct {
	UserID uuid.UUID
	Name   string
}

// Member is one row of the Team › Members table.
type Member struct {
	UserID       uuid.UUID
	Email        string
	Name         string
	AvatarURL    *string
	RoleID       uuid.UUID
	RoleName     string
	RoleKey      *string
	UsesSeat     bool
	Status       string
	Availability string
	LastActiveAt time.Time
	JoinedAt     time.Time
	TeamIDs      []uuid.UUID
}

// MemberFilter narrows the members list.
type MemberFilter struct {
	RoleID   *uuid.UUID
	TeamID   *uuid.UUID
	Status   *string
	Query    *string
	After    *MemberCursor
	PageSize int32
}

// MemberCursor is the last row of the previous page.
type MemberCursor struct {
	JoinedAt time.Time
	UserID   uuid.UUID
}

// MemberDetail is the member page: who they are and what they can do.
type MemberDetail struct {
	Member
	ChatScope     *string
	RoleChatScope string
	SuspendedAt   *time.Time
	SuspendReason *string
	AccessVersion int32
	NumberIDs     []uuid.UUID
	Overrides     []Override
	// Effective is the role's permissions plus active grants, minus active denies.
	Effective []string
}

// Override is one person's extra (grant) or removed (deny) permission.
type Override struct {
	ID            uuid.UUID
	PermissionKey string
	Effect        string
	Reason        string
	ExpiresAt     *time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
	GrantedBy     *uuid.UUID
	GrantedByName string
}

// Active reports whether the override applies now.
func (o Override) Active(now time.Time) bool {
	return o.RevokedAt == nil && (o.ExpiresAt == nil || o.ExpiresAt.After(now))
}

// OverrideInput grants or denies one permission to one member.
type OverrideInput struct {
	PermissionKey string
	Effect        string
	ExpiresAt     *time.Time
	Reason        string
}

// AccessInput changes a member's reach. A nil field is left as it is.
type AccessInput struct {
	// ChatScope "" means use the role's scope.
	ChatScope *string
	TeamIDs   []uuid.UUID
	NumberIDs []uuid.UUID
	SetTeams  bool
	SetNumber bool
}

// RemoveInput confirms a removal.
type RemoveInput struct {
	ConfirmName string
}

// Team groups members for routing and chat visibility.
type Team struct {
	ID            uuid.UUID
	Name          string
	Description   string
	Color         string
	NumberID      *uuid.UUID
	RoutingMethod string
	MemberIDs     []uuid.UUID
}

// Details are a team's editable fields, for creating or editing it.
type Details struct {
	Name          string
	Description   string
	Color         string
	NumberID      *uuid.UUID
	RoutingMethod string
}

// Seats is how many seats the plan has and how they're used.
type Seats struct {
	Limit     int32
	Used      int64
	Reserved  int64
	Available int64
}

// Invitation is one row of the pending invites list.
type Invitation struct {
	ID                 uuid.UUID
	Email              string
	RoleID             uuid.UUID
	RoleName           string
	Status             string
	ExpiresAt          time.Time
	LastSentAt         time.Time
	SendCount          int32
	RenewalRequestedAt *time.Time
	CreatedAt          time.Time
	InvitedByName      string
	TeamIDs            []uuid.UUID
}

// InviteInput is the Invite teammates wizard.
type InviteInput struct {
	Emails          []string
	RoleID          uuid.UUID
	TeamIDs         []uuid.UUID
	NumberIDs       []uuid.UUID
	ChatScope       *string
	Overrides       []OverrideInput
	PersonalMessage string
	ExpiresIn       string
	Require2FA      bool
}

// InvitePreview is what the invite link shows before joining.
type InvitePreview struct {
	State          string
	WorkspaceName  string
	Email          string
	RoleName       string
	InvitedByName  string
	InvitedByRole  string
	ExpiresAt      time.Time
	Message        string
	CanDo          []string
	WorkspaceID    uuid.UUID
	PermissionKeys []string
	TeamNames      []string
	// NumberLabels are "Main line (+1 415 555 0142)".
	NumberLabels []string
}

// LogSummary is the access log's headline numbers.
type LogSummary struct {
	Events            int64
	PermissionChanges int64
	Expiring          []Expiring
	Alerts            []LoginAlert
}

// Expiring is a temporary permission ending soon.
type Expiring struct {
	UserID        uuid.UUID
	Name          string
	PermissionKey string
	ExpiresAt     time.Time
}

// LoginAlert is a member with repeated failed sign-ins.
type LoginAlert struct {
	UserID   uuid.UUID
	Name     string
	Failures int64
	LastAt   time.Time
}

// LogEntry is one row of the access log.
type LogEntry struct {
	ID           uuid.UUID
	CreatedAt    time.Time
	ActorID      *uuid.UUID
	ActorName    string
	SubjectID    *uuid.UUID
	SubjectName  string
	SubjectEmail *string
	Action       string
	Target       string
	Before       []byte
	After        []byte
	Note         string
	IP           string
	UserAgent    string
}

// LogFilter narrows the access log.
type LogFilter struct {
	Action    *string
	SubjectID *uuid.UUID
	Since     time.Time
	Before    *LogCursor
	PageSize  int32
}

// LogCursor is the last row of the previous page.
type LogCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

// Meta describes the request behind a change, for the access log.
type Meta struct {
	IP        string
	UserAgent string
}
