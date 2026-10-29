package team

import (
	"time"

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

// Permission is one entry of the catalog, as the Roles editor shows it.
type Permission struct {
	Key       string
	Group     string
	Label     string
	Sensitive bool
}

// Seats is how many seats the plan has and how they're used.
type Seats struct {
	Limit     int32
	Used      int64
	Reserved  int64
	Available int64
}

// Meta describes the request behind a change, for the access log.
type Meta struct {
	IP        string
	UserAgent string
}
