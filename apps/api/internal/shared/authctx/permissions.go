package authctx

import "github.com/google/uuid"

// Permission is one thing a workspace member can be allowed to do. Its value
// is a fixed bit position (permissions.bit in the database); a bit is never
// reused, so a retired permission keeps its constant.
type Permission uint8

// The permission catalog. Keep in step with migration 00010 (a test checks it).
const (
	PermInboxReply          Permission = 0
	PermInboxSendTemplate   Permission = 1
	PermInboxCloseReassign  Permission = 2
	PermInboxNotes          Permission = 3
	PermContactsView        Permission = 4
	PermContactsEdit        Permission = 5
	PermContactsImport      Permission = 6
	PermContactsExport      Permission = 7
	PermContactsDelete      Permission = 8
	PermBroadcastsCreate    Permission = 9
	PermBroadcastsSend      Permission = 10
	PermBroadcastsReports   Permission = 11
	PermTemplatesCreate     Permission = 12
	PermTemplatesSubmit     Permission = 13
	PermTemplatesDelete     Permission = 14
	PermAutomationsEdit     Permission = 15
	PermAutomationsPublish  Permission = 16
	PermIntegrationsManage  Permission = 17
	PermIntegrationsAPIKeys Permission = 18
	PermTeamInvite          Permission = 19
	PermTeamRoles           Permission = 20
	PermTeamRemove          Permission = 21
	PermBillingView         Permission = 22
	PermBillingPay          Permission = 23
	PermBillingTopUp        Permission = 24
	permissionCount                    = 25
)

var permissionKeys = [permissionCount]string{
	"inbox.reply", "inbox.send_template", "inbox.close_reassign", "inbox.notes",
	"contacts.view", "contacts.edit", "contacts.import", "contacts.export", "contacts.delete",
	"broadcasts.create", "broadcasts.send", "broadcasts.reports",
	"templates.create", "templates.submit", "templates.delete",
	"automations.edit", "automations.publish",
	"integrations.manage", "integrations.api_keys",
	"team.invite", "team.roles", "team.remove",
	"billing.view", "billing.pay", "billing.topup",
}

// Key is the permission's stable name, e.g. "contacts.export".
func (p Permission) Key() string {
	if int(p) >= permissionCount {
		return ""
	}
	return permissionKeys[p]
}

// PermissionByKey looks a permission up by its key.
func PermissionByKey(key string) (Permission, bool) {
	for i, k := range permissionKeys {
		if k == key {
			return Permission(i), true
		}
	}
	return 0, false
}

// AllPermissions returns every permission in catalog order.
func AllPermissions() []Permission {
	out := make([]Permission, permissionCount)
	for i := range out {
		out[i] = Permission(i)
	}
	return out
}

// Bits is a set of permissions, one bit each.
type Bits uint64

// AllBits is every permission; the Owner always has it.
const AllBits = Bits(1)<<permissionCount - 1

// Has reports whether p is in the set.
func (b Bits) Has(p Permission) bool { return b&(1<<p) != 0 }

// With returns the set with p added.
func (b Bits) With(p Permission) Bits { return b | 1<<p }

// Keys lists the set's permission keys in catalog order.
func (b Bits) Keys() []string {
	out := make([]string, 0, permissionCount)
	for i, k := range permissionKeys {
		if b.Has(Permission(i)) {
			out = append(out, k)
		}
	}
	return out
}

// Chat scopes: which conversations a member can open.
const (
	ScopeAssigned = "assigned"
	ScopeTeam     = "team"
	ScopeAll      = "all"
)

// Access is what one member can do in one workspace, resolved once per request
// by middleware.RequireWorkspace.
type Access struct {
	WorkspaceID   uuid.UUID
	UserID        uuid.UUID
	RoleID        uuid.UUID
	Owner         bool
	Bits          Bits
	Scope         string
	Teams         []uuid.UUID
	Numbers       []uuid.UUID
	RoleVersion   int32
	AccessVersion int32
}

// Can reports whether the member has p.
func (a Access) Can(p Permission) bool { return a.Owner || a.Bits.Has(p) }
