package team

import (
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// ---------- shared inputs ----------

type workspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type memberPath struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
}

// metaHeaders is mixed into inputs that change access, for the access log.
type metaHeaders struct {
	UserAgent string `header:"User-Agent"`
}

// ---------- access ----------

type accessDTO struct {
	RoleID        uuid.UUID   `json:"role_id"`
	IsOwner       bool        `json:"is_owner"`
	Permissions   []string    `json:"permissions" doc:"Every permission key the member has now"`
	ChatScope     string      `json:"chat_scope" enum:"assigned,team,all"`
	TeamIDs       []uuid.UUID `json:"team_ids"`
	NumberIDs     []uuid.UUID `json:"number_ids"`
	AccessVersion string      `json:"access_version" doc:"Same value as the X-Access-Version header"`
}

type accessOutput struct {
	Body accessDTO
}

func toAccessDTO(a authctx.Access) accessDTO {
	bits := a.Bits
	if a.Owner {
		bits = authctx.AllBits
	}
	return accessDTO{
		RoleID: a.RoleID, IsOwner: a.Owner, Permissions: bits.Keys(), ChatScope: a.Scope,
		TeamIDs: nonNilIDs(a.Teams), NumberIDs: nonNilIDs(a.Numbers),
		AccessVersion: itoa(a.RoleVersion) + "." + itoa(a.AccessVersion),
	}
}

type availabilityInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Availability string `json:"availability" enum:"online,away,offline"`
	}
}

type leaveInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
}

type permissionDTO struct {
	Key       string `json:"key" example:"contacts.export"`
	Group     string `json:"group" example:"contacts"`
	Label     string `json:"label" example:"Export contacts"`
	Sensitive bool   `json:"sensitive" doc:"Show a warning before giving it"`
}

type permissionsOutput struct {
	Body struct {
		Permissions []permissionDTO `json:"permissions"`
	}
}

type seatsDTO struct {
	Limit     int32 `json:"limit"`
	Used      int64 `json:"used" doc:"Members whose role uses a seat"`
	Reserved  int64 `json:"reserved" doc:"Pending invites whose role uses a seat"`
	Available int64 `json:"available"`
}

type seatsOutput struct {
	Body seatsDTO
}

// ---------- roles ----------

type roleDTO struct {
	ID          uuid.UUID `json:"id"`
	SystemKey   *string   `json:"system_key" nullable:"true" enum:"owner,admin,agent,viewer" doc:"Null for a custom role"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color" enum:"green,blue,amber,red,gray"`
	ChatScope   string    `json:"chat_scope" enum:"assigned,team,all"`
	UsesSeat    bool      `json:"uses_seat"`
	Locked      bool      `json:"locked" doc:"The Owner role can't be edited"`
	Version     int32     `json:"version"`
	Permissions []string  `json:"permissions"`
	MemberCount int64     `json:"member_count"`
}

func toRoleDTO(r Role) roleDTO {
	d := roleDTO(r)
	if d.Permissions == nil {
		d.Permissions = []string{}
	}
	return d
}

type rolesOutput struct {
	Body struct {
		Roles []roleDTO `json:"roles"`
	}
}

type roleOutput struct {
	Body roleDTO
}

type roleBody struct {
	Name            string     `json:"name,omitempty" maxLength:"50"`
	Description     string     `json:"description,omitempty" maxLength:"200"`
	Color           string     `json:"color,omitempty" enum:"green,blue,amber,red,gray"`
	ChatScope       string     `json:"chat_scope,omitempty" enum:"assigned,team,all"`
	Permissions     []string   `json:"permissions,omitempty" doc:"Omit to keep the current set (or copy_from's)"`
	CopyFrom        *uuid.UUID `json:"copy_from_role_id,omitempty" doc:"Start a new role from another role"`
	ExpectedVersion int32      `json:"expected_version,omitempty" doc:"The version you edited; 409 if it changed"`
	NotifyMembers   bool       `json:"notify_members,omitempty"`
}

func (b roleBody) input() RoleInput {
	return RoleInput(b)
}

type createRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body roleBody
}

type updateRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      uuid.UUID `path:"roleId"`
	metaHeaders
	Body roleBody
}

type resetRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      uuid.UUID `path:"roleId"`
	metaHeaders
}

type deleteRoleInput struct {
	WorkspaceID  uuid.UUID `path:"workspaceId"`
	RoleID       uuid.UUID `path:"roleId"`
	MoveToRoleID uuid.UUID `query:"move_to_role_id" required:"true" doc:"Where its members and pending invites go"`
	metaHeaders
}

type memberRefDTO struct {
	UserID uuid.UUID `json:"user_id"`
	Name   string    `json:"name"`
}

type changeDTO struct {
	Gains         []string       `json:"gains" doc:"Permission keys, or a chat access label"`
	Losses        []string       `json:"losses"`
	SensitiveGain bool           `json:"sensitive_gain"`
	Affected      []memberRefDTO `json:"affected"`
}

type changeOutput struct {
	Body changeDTO
}

func toChangeDTO(c RoleChange) changeDTO {
	d := changeDTO{Gains: nonNil(c.Gains), Losses: nonNil(c.Losses), SensitiveGain: c.SensitiveGain, Affected: []memberRefDTO{}}
	for _, m := range c.Affected {
		d.Affected = append(d.Affected, memberRefDTO(m))
	}
	return d
}

// ---------- members ----------

// MemberDTO is exported so huma flattens it into memberDetailDTO (it skips unexported embeds).
type MemberDTO struct {
	UserID       uuid.UUID   `json:"user_id"`
	Email        string      `json:"email"`
	Name         string      `json:"name"`
	AvatarURL    *string     `json:"avatar_url"`
	RoleID       uuid.UUID   `json:"role_id"`
	RoleName     string      `json:"role_name"`
	RoleKey      *string     `json:"role_key" nullable:"true" enum:"owner,admin,agent,viewer" doc:"Null for a custom role"`
	UsesSeat     bool        `json:"uses_seat"`
	Status       string      `json:"status" enum:"active,suspended"`
	Availability string      `json:"availability" enum:"online,away,offline"`
	LastActiveAt time.Time   `json:"last_active_at"`
	JoinedAt     time.Time   `json:"joined_at"`
	TeamIDs      []uuid.UUID `json:"team_ids"`
}

func toMemberDTO(m Member) MemberDTO {
	d := MemberDTO(m)
	d.TeamIDs = nonNilIDs(d.TeamIDs)
	return d
}

type listMembersInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	RoleID      string    `query:"role_id" format:"uuid"`
	TeamID      string    `query:"team_id" format:"uuid"`
	Status      string    `query:"status" enum:"active,suspended"`
	Query       string    `query:"q" maxLength:"100"`
	Cursor      string    `query:"cursor" doc:"next_cursor from the previous page"`
	PageSize    int32     `query:"page_size" minimum:"1" maximum:"500" default:"100"`
}

type membersOutput struct {
	Body struct {
		Members    []MemberDTO `json:"members"`
		NextCursor *string     `json:"next_cursor"`
	}
}

type overrideDTO struct {
	ID            uuid.UUID  `json:"id"`
	PermissionKey string     `json:"permission_key"`
	Effect        string     `json:"effect" enum:"grant,deny"`
	Reason        string     `json:"reason"`
	ExpiresAt     *time.Time `json:"expires_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	GrantedBy     *uuid.UUID `json:"granted_by" nullable:"true"`
	GrantedByName string     `json:"granted_by_name"`
	Active        bool       `json:"active"`
}

type memberDetailDTO struct {
	MemberDTO
	ChatScope     *string       `json:"chat_scope" nullable:"true" enum:"assigned,team,all" doc:"Null means the role's scope"`
	RoleChatScope string        `json:"role_chat_scope" enum:"assigned,team,all"`
	SuspendedAt   *time.Time    `json:"suspended_at"`
	SuspendReason *string       `json:"suspend_reason"`
	AccessVersion int32         `json:"access_version"`
	NumberIDs     []uuid.UUID   `json:"number_ids"`
	Overrides     []overrideDTO `json:"overrides" doc:"Empty unless you can change roles"`
	Effective     []string      `json:"effective_permissions"`
}

type memberOutput struct {
	Body memberDetailDTO
}

func toMemberDetailDTO(d MemberDetail, now time.Time) memberDetailDTO {
	out := memberDetailDTO{
		MemberDTO: toMemberDTO(d.Member), ChatScope: d.ChatScope, RoleChatScope: d.RoleChatScope,
		SuspendedAt: d.SuspendedAt, SuspendReason: d.SuspendReason, AccessVersion: d.AccessVersion,
		NumberIDs: nonNilIDs(d.NumberIDs), Overrides: []overrideDTO{}, Effective: nonNil(d.Effective),
	}
	for _, o := range d.Overrides {
		out.Overrides = append(out.Overrides, overrideDTO{
			ID: o.ID, PermissionKey: o.PermissionKey, Effect: o.Effect, Reason: o.Reason, ExpiresAt: o.ExpiresAt,
			RevokedAt: o.RevokedAt, CreatedAt: o.CreatedAt, GrantedBy: o.GrantedBy, GrantedByName: o.GrantedByName,
			Active: o.Active(now),
		})
	}
	return out
}

type memberRoleInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
	Body struct {
		RoleID uuid.UUID `json:"role_id"`
	}
}

type memberAccessInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
	Body struct {
		ChatScope *string      `json:"chat_scope,omitempty" doc:"\"assigned\", \"team\" or \"all\"; \"\" to use the role's scope; omit to keep"`
		TeamIDs   *[]uuid.UUID `json:"team_ids,omitempty" doc:"Replaces the member's teams; omit to keep"`
		NumberIDs *[]uuid.UUID `json:"number_ids,omitempty" doc:"Replaces the member's WhatsApp numbers; omit to keep"`
	}
}

type overrideBody struct {
	PermissionKey string     `json:"permission_key"`
	Effect        string     `json:"effect" enum:"grant,deny" default:"grant"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty" doc:"Removed automatically at this time"`
	Reason        string     `json:"reason,omitempty" maxLength:"200" doc:"Shown in the access log"`
}

func (b overrideBody) input() OverrideInput {
	return OverrideInput(b)
}

type addOverrideInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
	Body overrideBody
}

type revokeOverrideInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	OverrideID  uuid.UUID `path:"overrideId"`
	metaHeaders
}

type suspendInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
	Body struct {
		Reason string `json:"reason,omitempty" maxLength:"200"`
	}
}

type memberActionInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
}

type removeInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	UserID      uuid.UUID `path:"userId"`
	metaHeaders
	Body struct {
		ConfirmName string `json:"confirm_name" doc:"The member's name, typed to confirm"`
	}
}

// ---------- teams ----------

type teamDTO struct {
	ID            uuid.UUID   `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Color         string      `json:"color" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID  `json:"number_id" nullable:"true"`
	RoutingMethod string      `json:"routing_method" enum:"round_robin,least_busy,manual"`
	MemberIDs     []uuid.UUID `json:"member_ids"`
}

func toTeamDTO(t Team) teamDTO {
	d := teamDTO(t)
	d.MemberIDs = nonNilIDs(d.MemberIDs)
	return d
}

type teamsOutput struct {
	Body struct {
		Teams []teamDTO `json:"teams"`
	}
}

type teamOutput struct {
	Body teamDTO
}

type teamBody struct {
	Name          string     `json:"name" minLength:"1" maxLength:"50"`
	Description   string     `json:"description,omitempty" maxLength:"200"`
	Color         string     `json:"color,omitempty" enum:"green,blue,amber,red,gray"`
	NumberID      *uuid.UUID `json:"number_id,omitempty"`
	RoutingMethod string     `json:"routing_method,omitempty" enum:"round_robin,least_busy,manual"`
}

func (b teamBody) input() Details {
	return Details(b)
}

type createTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body teamBody
}

type updateTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
	Body teamBody
}

type teamMembersInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
	Body struct {
		UserIDs []uuid.UUID `json:"user_ids" maxItems:"5000"`
	}
}

type deleteTeamInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	TeamID      uuid.UUID `path:"teamId"`
	metaHeaders
}

// ---------- invitations ----------

type invitationDTO struct {
	ID                 uuid.UUID   `json:"id"`
	Email              string      `json:"email"`
	RoleID             uuid.UUID   `json:"role_id"`
	RoleName           string      `json:"role_name"`
	Status             string      `json:"status" enum:"pending,accepted,revoked,expired"`
	ExpiresAt          time.Time   `json:"expires_at"`
	LastSentAt         time.Time   `json:"last_sent_at"`
	SendCount          int32       `json:"send_count"`
	RenewalRequestedAt *time.Time  `json:"renewal_requested_at" doc:"The invitee asked for a new link"`
	CreatedAt          time.Time   `json:"created_at"`
	InvitedByName      string      `json:"invited_by_name"`
	TeamIDs            []uuid.UUID `json:"team_ids"`
}

func toInvitationDTO(i Invitation) invitationDTO {
	d := invitationDTO(i)
	d.TeamIDs = nonNilIDs(d.TeamIDs)
	return d
}

type invitationsOutput struct {
	Body struct {
		Invitations []invitationDTO `json:"invitations"`
	}
}

type inviteInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	metaHeaders
	Body struct {
		Emails          []string       `json:"emails" minItems:"1" maxItems:"50"`
		RoleID          uuid.UUID      `json:"role_id"`
		TeamIDs         []uuid.UUID    `json:"team_ids,omitempty"`
		NumberIDs       []uuid.UUID    `json:"number_ids,omitempty"`
		ChatScope       *string        `json:"chat_scope,omitempty" enum:"assigned,team,all" doc:"Omit to use the role's scope"`
		Overrides       []overrideBody `json:"overrides,omitempty" doc:"Changes for these people only; the role stays the same"`
		PersonalMessage string         `json:"personal_message,omitempty" maxLength:"500"`
		ExpiresIn       string         `json:"expires_in,omitempty" enum:"24h,7d,30d" default:"7d"`
		Require2FA      bool           `json:"require_2fa,omitempty"`
	}
}

type invitationActionInput struct {
	WorkspaceID  uuid.UUID `path:"workspaceId"`
	InvitationID uuid.UUID `path:"invitationId"`
	metaHeaders
}

type tokenInput struct {
	metaHeaders
	Body struct {
		Token string `json:"token" minLength:"20" maxLength:"100" doc:"From the invite link's #t= fragment"`
	}
}

type previewDTO struct {
	State         string    `json:"state" enum:"valid,expired,revoked,accepted"`
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	WorkspaceName string    `json:"workspace_name"`
	Email         string    `json:"email" doc:"The address the invite is for"`
	RoleName      string    `json:"role_name"`
	InvitedByName string    `json:"invited_by_name"`
	InvitedByRole string    `json:"invited_by_role"`
	ExpiresAt     time.Time `json:"expires_at"`
	Message       string    `json:"message"`
	CanDo         []string  `json:"can_do" doc:"Labels of what they'll be able to do; empty unless valid"`
	Permissions   []string  `json:"permissions"`
	TeamNames     []string  `json:"team_names"`
	NumberLabels  []string  `json:"number_labels"`
}

type previewOutput struct {
	Body previewDTO
}

type acceptOutput struct {
	Body struct {
		WorkspaceID uuid.UUID `json:"workspace_id"`
	}
}

// ---------- access log ----------

type logDTO struct {
	ID           uuid.UUID       `json:"id"`
	CreatedAt    time.Time       `json:"created_at"`
	ActorID      *uuid.UUID      `json:"actor_id" doc:"Null for the system" nullable:"true"`
	ActorName    string          `json:"actor_name"`
	SubjectID    *uuid.UUID      `json:"subject_id" nullable:"true"`
	SubjectName  string          `json:"subject_name"`
	SubjectEmail *string         `json:"subject_email"`
	Action       string          `json:"action"`
	Target       string          `json:"target"`
	Before       json.RawMessage `json:"before" nullable:"true"`
	After        json.RawMessage `json:"after" nullable:"true"`
	Note         string          `json:"note"`
	IP           string          `json:"ip"`
	UserAgent    string          `json:"user_agent"`
}

type listLogInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Action      string    `query:"action" maxLength:"40"`
	SubjectID   string    `query:"member" format:"uuid"`
	Since       time.Time `query:"since" doc:"Oldest entry to include; at most 12 months back"`
	Cursor      string    `query:"cursor"`
	PageSize    int32     `query:"page_size" minimum:"1" maximum:"200" default:"50"`
}

type logOutput struct {
	Body struct {
		Entries    []logDTO `json:"entries"`
		NextCursor *string  `json:"next_cursor"`
	}
}

func toLogDTO(e LogEntry) logDTO {
	return logDTO{
		ID: e.ID, CreatedAt: e.CreatedAt, ActorID: e.ActorID, ActorName: e.ActorName, SubjectID: e.SubjectID,
		SubjectName: e.SubjectName, SubjectEmail: e.SubjectEmail, Action: e.Action, Target: e.Target,
		Before: rawOrNil(e.Before), After: rawOrNil(e.After), Note: e.Note, IP: e.IP, UserAgent: e.UserAgent,
	}
}

// ---------- helpers ----------

// encodeCursor packs a (time, id) pair into an opaque string.
func encodeCursor(t time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeCursor(s string) (time.Time, uuid.UUID, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	ts, id, ok := strings.Cut(string(b), "|")
	if !ok {
		return time.Time{}, uuid.Nil, false
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}
	u, err := uuid.Parse(id)
	return t, u, err == nil
}

func optionalID(s string) *uuid.UUID {
	if id, err := uuid.Parse(s); err == nil {
		return &id
	}
	return nil
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s != "" {
		return &s
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nonNilIDs(s []uuid.UUID) []uuid.UUID {
	if s == nil {
		return []uuid.UUID{}
	}
	return s
}

func rawOrNil(b []byte) json.RawMessage {
	if len(b) == 0 {
		return nil
	}
	return json.RawMessage(b)
}

func itoa(n int32) string { return strconv.FormatInt(int64(n), 10) }

type expiringDTO struct {
	UserID        uuid.UUID `json:"user_id"`
	Name          string    `json:"name"`
	PermissionKey string    `json:"permission_key"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type loginAlertDTO struct {
	UserID   uuid.UUID `json:"user_id"`
	Name     string    `json:"name"`
	Failures int64     `json:"failures"`
	LastAt   time.Time `json:"last_at"`
}

type logSummaryOutput struct {
	Body struct {
		Events            int64           `json:"events" doc:"Access log entries in the last 30 days"`
		PermissionChanges int64           `json:"permission_changes"`
		Expiring          []expiringDTO   `json:"expiring" doc:"Temporary permissions ending within 7 days"`
		Alerts            []loginAlertDTO `json:"alerts" doc:"Members with 3+ failed sign-ins in 30 days"`
	}
}
