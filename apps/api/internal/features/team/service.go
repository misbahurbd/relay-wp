// Package team owns who can do what in a workspace: roles, members and their
// per-person permissions, teams, invitations and the access log
// (docs/PHASE-3-TEAM-ACCESS.md).
package team

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Workspaces reads the plan's seat limit. Implemented by workspaces.Directory.
type Workspaces interface {
	// LockSeatLimit locks the workspace row (in db's transaction) so seat checks run one at a time.
	LockSeatLimit(ctx context.Context, db database.DBTX, workspaceID uuid.UUID) (int32, error)
}

// AccountReader returns a user's email and whether it's verified. Implemented by auth.Accounts.
type AccountReader interface {
	AccountInfo(ctx context.Context, userID uuid.UUID) (email string, verified, hasPassword bool, err error)
}

// Enqueuer inserts background jobs in a transaction.
type Enqueuer interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Config holds team settings from the environment.
type Config struct {
	// AppBaseURL builds invite links.
	AppBaseURL string
}

// Service manages roles, members, teams and invitations.
type Service struct {
	pool       *pgxpool.Pool
	workspaces Workspaces
	accounts   AccountReader
	queue      Enqueuer
	cfg        Config
	now        func() time.Time
}

// New builds the service.
func New(pool *pgxpool.Pool, workspaces Workspaces, accounts AccountReader, queue Enqueuer, cfg Config) *Service {
	return &Service{
		pool: pool, workspaces: workspaces, accounts: accounts, queue: queue, cfg: cfg,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// Seed gives a new workspace its four system roles and makes ownerID its Owner.
// workspaces.Service calls it inside the create-workspace transaction.
func (s *Service) Seed(ctx context.Context, tx pgx.Tx, workspaceID, ownerID uuid.UUID) error {
	r := newRepository(tx)
	var ownerRole uuid.UUID
	for _, sr := range systemRoles {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("role id: %w", err)
		}
		key := sr.key
		if err := r.CreateRole(ctx, gen.CreateRoleParams{
			ID: id, WorkspaceID: workspaceID, SystemKey: &key, Name: sr.name, Description: sr.description,
			Color: sr.color, ChatScope: sr.scope, UsesSeat: sr.key != RoleViewer, Locked: sr.key == RoleOwner,
		}); err != nil {
			return fmt.Errorf("create %s role: %w", sr.key, err)
		}
		if err := r.SetRolePermissions(ctx, gen.SetRolePermissionsParams{
			WorkspaceID: workspaceID, RoleID: id, Keys: keysOf(sr.perms),
		}); err != nil {
			return fmt.Errorf("%s role permissions: %w", sr.key, err)
		}
		if sr.key == RoleOwner {
			ownerRole = id
		}
	}
	if err := r.AddMember(ctx, gen.AddMemberParams{WorkspaceID: workspaceID, UserID: ownerID, RoleID: ownerRole}); err != nil {
		return fmt.Errorf("add owner: %w", err)
	}
	return nil
}

// Permissions returns the catalog in editor order.
func (s *Service) Permissions(ctx context.Context) ([]Permission, error) {
	rows, err := newRepository(s.pool).ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}
	out := make([]Permission, len(rows))
	for i, p := range rows {
		out[i] = Permission{Key: p.Key, Group: p.GroupKey, Label: p.Label, Sensitive: p.Sensitive}
	}
	return out, nil
}

// Seats returns the plan's seats and how many are used or held by pending invites.
func (s *Service) Seats(ctx context.Context, workspaceID uuid.UUID) (Seats, error) {
	var out Seats
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		out, err = s.seats(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// seats locks the workspace row; call it in the transaction that adds seats.
func (s *Service) seats(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID) (Seats, error) {
	limit, err := s.workspaces.LockSeatLimit(ctx, tx, workspaceID)
	if err != nil {
		return Seats{}, err
	}
	used, err := newRepository(tx).CountSeatsUsed(ctx, workspaceID)
	if err != nil {
		return Seats{}, fmt.Errorf("count seats: %w", err)
	}
	return Seats{
		Limit: limit, Used: used.Members, Reserved: used.Reserved,
		Available: max(0, int64(limit)-used.Members-used.Reserved),
	}, nil
}

// ---------- roles ----------

// ListRoles returns the system roles, then custom roles by name.
func (s *Service) ListRoles(ctx context.Context, workspaceID uuid.UUID) ([]Role, error) {
	rows, err := newRepository(s.pool).ListRoles(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	out := make([]Role, len(rows))
	for i, r := range rows {
		out[i] = Role(r)
	}
	return out, nil
}

func (s *Service) getRole(ctx context.Context, db database.DBTX, workspaceID, roleID uuid.UUID) (Role, error) {
	row, err := newRepository(db).GetRole(ctx, gen.GetRoleParams{WorkspaceID: workspaceID, RoleID: roleID})
	if isNoRows(err) {
		return Role{}, ErrRoleNotFound
	}
	if err != nil {
		return Role{}, fmt.Errorf("get role: %w", err)
	}
	return Role(row), nil
}

// CreateRole adds a custom role, optionally copying another role's access.
func (s *Service) CreateRole(ctx context.Context, actor authctx.Access, meta Meta, in RoleInput) (Role, error) {
	if in.CopyFrom != nil {
		src, err := s.getRole(ctx, s.pool, actor.WorkspaceID, *in.CopyFrom)
		if err != nil {
			return Role{}, err
		}
		if in.Permissions == nil {
			in.Permissions = src.Permissions
		}
		if in.ChatScope == "" {
			in.ChatScope = src.ChatScope
		}
	}
	if in.Color == "" {
		in.Color = "blue"
	}
	if err := validateRole(in, true); err != nil {
		return Role{}, err
	}
	if err := canGive(actor, in.Permissions); err != nil {
		return Role{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Role{}, fmt.Errorf("role id: %w", err)
	}
	name := strings.TrimSpace(in.Name)
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		err := r.CreateRole(ctx, gen.CreateRoleParams{
			ID: id, WorkspaceID: actor.WorkspaceID, Name: name, Description: strings.TrimSpace(in.Description),
			Color: in.Color, ChatScope: in.ChatScope, UsesSeat: true, CreatedBy: &actor.UserID,
		})
		if isUnique(err, "roles_name_idx") {
			return ErrRoleName
		}
		if err != nil {
			return fmt.Errorf("create role: %w", err)
		}
		if err := r.SetRolePermissions(ctx, gen.SetRolePermissionsParams{
			WorkspaceID: actor.WorkspaceID, RoleID: id, Keys: in.Permissions,
		}); err != nil {
			return fmt.Errorf("role permissions: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionRoleCreated, target: id.String(),
			after: map[string]any{"name": name, "permissions": in.Permissions, "chat_scope": in.ChatScope},
		}, meta)
	})
	if err != nil {
		return Role{}, err
	}
	return s.getRole(ctx, s.pool, actor.WorkspaceID, id)
}

// PreviewRole says what saving in would give and take away, and from whom.
func (s *Service) PreviewRole(ctx context.Context, workspaceID, roleID uuid.UUID, in RoleInput) (RoleChange, error) {
	role, err := s.getRole(ctx, s.pool, workspaceID, roleID)
	if err != nil {
		return RoleChange{}, err
	}
	if in.Permissions == nil {
		in.Permissions = role.Permissions
	}
	if in.ChatScope == "" {
		in.ChatScope = role.ChatScope
	}
	if err := validatePermissions(in.Permissions); err != nil {
		return RoleChange{}, err
	}
	change := diffPermissions(role.Permissions, in.Permissions)
	if in.ChatScope != role.ChatScope {
		label := "Chat access: " + scopeLabel(in.ChatScope)
		if scopeRank(in.ChatScope) > scopeRank(role.ChatScope) {
			change.Gains = append(change.Gains, label)
		} else {
			change.Losses = append(change.Losses, label)
		}
	}
	members, _, err := s.ListMembers(ctx, workspaceID, MemberFilter{RoleID: &roleID, PageSize: 500})
	if err != nil {
		return RoleChange{}, err
	}
	for _, m := range members {
		change.Affected = append(change.Affected, MemberRef{UserID: m.UserID, Name: m.Name})
	}
	return change, nil
}

// UpdateRole saves an edit from the Roles editor. Everyone with the role gets
// the change on their next request.
func (s *Service) UpdateRole(ctx context.Context, actor authctx.Access, meta Meta, roleID uuid.UUID, in RoleInput) (Role, error) {
	if roleID == actor.RoleID && !actor.Owner {
		return Role{}, ErrCannotChangeSelf
	}
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		version, err := r.LockRole(ctx, gen.LockRoleParams{WorkspaceID: actor.WorkspaceID, RoleID: roleID})
		if isNoRows(err) {
			return ErrRoleNotFound
		}
		if err != nil {
			return fmt.Errorf("lock role: %w", err)
		}
		if in.ExpectedVersion != 0 && in.ExpectedVersion != version {
			return ErrRoleChanged
		}
		role, err := s.getRole(ctx, tx, actor.WorkspaceID, roleID)
		if err != nil {
			return err
		}
		if role.Locked {
			return ErrRoleLocked
		}
		if role.SystemKey != nil || in.Name == "" {
			in.Name = role.Name // system roles keep their names
		}
		if in.Description == "" {
			in.Description = role.Description
		}
		if in.Permissions == nil {
			in.Permissions = role.Permissions
		}
		if in.ChatScope == "" {
			in.ChatScope = role.ChatScope
		}
		if in.Color == "" {
			in.Color = role.Color
		}
		if err := validateRole(in, role.SystemKey == nil); err != nil {
			return err
		}
		change := diffPermissions(role.Permissions, in.Permissions)
		if err := canGive(actor, change.Gains); err != nil {
			return err
		}
		return s.saveRole(ctx, r, actor, meta, role, in)
	})
	if err != nil {
		return Role{}, err
	}
	return s.getRole(ctx, s.pool, actor.WorkspaceID, roleID)
}

// ResetRole restores a system role's default permissions and scope.
func (s *Service) ResetRole(ctx context.Context, actor authctx.Access, meta Meta, roleID uuid.UUID) (Role, error) {
	if roleID == actor.RoleID && !actor.Owner {
		return Role{}, ErrCannotChangeSelf
	}
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		if _, err := r.LockRole(ctx, gen.LockRoleParams{WorkspaceID: actor.WorkspaceID, RoleID: roleID}); isNoRows(err) {
			return ErrRoleNotFound
		} else if err != nil {
			return fmt.Errorf("lock role: %w", err)
		}
		role, err := s.getRole(ctx, tx, actor.WorkspaceID, roleID)
		if err != nil {
			return err
		}
		if role.Locked {
			return ErrRoleLocked
		}
		if role.SystemKey == nil {
			return ErrNotSystem
		}
		def, _ := systemRoleDefaults(*role.SystemKey)
		in := RoleInput{Name: role.Name, Description: def.description, Color: def.color, ChatScope: def.scope, Permissions: keysOf(def.perms)}
		if err := canGive(actor, diffPermissions(role.Permissions, in.Permissions).Gains); err != nil {
			return err
		}
		return s.saveRole(ctx, r, actor, meta, role, in)
	})
	if err != nil {
		return Role{}, err
	}
	return s.getRole(ctx, s.pool, actor.WorkspaceID, roleID)
}

func (s *Service) saveRole(ctx context.Context, r repository, actor authctx.Access, meta Meta, role Role, in RoleInput) error {
	if err := r.UpdateRole(ctx, gen.UpdateRoleParams{
		WorkspaceID: actor.WorkspaceID, RoleID: role.ID, Name: strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description), Color: in.Color, ChatScope: in.ChatScope,
	}); isUnique(err, "roles_name_idx") {
		return ErrRoleName
	} else if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if err := r.SetRolePermissions(ctx, gen.SetRolePermissionsParams{
		WorkspaceID: actor.WorkspaceID, RoleID: role.ID, Keys: in.Permissions,
	}); err != nil {
		return fmt.Errorf("role permissions: %w", err)
	}
	change := diffPermissions(role.Permissions, in.Permissions)
	return r.log(ctx, event{
		workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionRoleUpdated, target: role.ID.String(),
		before: map[string]any{"name": role.Name, "chat_scope": role.ChatScope, "permissions": role.Permissions},
		after:  map[string]any{"name": in.Name, "chat_scope": in.ChatScope, "permissions": in.Permissions},
		note:   fmt.Sprintf("%d added, %d removed, %d members affected", len(change.Gains), len(change.Losses), role.MemberCount),
	}, meta)
}

// DeleteRole removes a custom role after moving its members and pending invites to moveTo.
func (s *Service) DeleteRole(ctx context.Context, actor authctx.Access, meta Meta, roleID, moveTo uuid.UUID) error {
	if roleID == actor.RoleID {
		return ErrCannotChangeSelf
	}
	if roleID == moveTo {
		return ErrInvalidInput.WithDetails(apperr.Detail{Field: "move_to_role_id", Message: "Choose a different role"})
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := newRepository(tx)
		role, err := s.getRole(ctx, tx, actor.WorkspaceID, roleID)
		if err != nil {
			return err
		}
		if role.SystemKey != nil {
			return ErrSystemRole
		}
		target, err := s.getRole(ctx, tx, actor.WorkspaceID, moveTo)
		if isNoRows(err) || apperr.IsKind(err, apperr.KindNotFound) {
			return ErrInvalidInput.WithDetails(apperr.Detail{Field: "move_to_role_id", Message: "Choose where its members go"})
		}
		if err != nil {
			return err
		}
		if target.SystemKey != nil && *target.SystemKey == RoleOwner {
			return ErrOwnerRole
		}
		if err := canGive(actor, target.Permissions); err != nil {
			return err
		}
		moved, err := r.MoveRoleMembers(ctx, gen.MoveRoleMembersParams{WorkspaceID: actor.WorkspaceID, FromRoleID: roleID, ToRoleID: moveTo})
		if err != nil {
			return fmt.Errorf("move members: %w", err)
		}
		if err := r.MoveRoleInvitations(ctx, gen.MoveRoleInvitationsParams{WorkspaceID: actor.WorkspaceID, FromRoleID: roleID, ToRoleID: moveTo}); err != nil {
			return fmt.Errorf("move invites: %w", err)
		}
		if _, err := r.DeleteRole(ctx, gen.DeleteRoleParams{WorkspaceID: actor.WorkspaceID, RoleID: roleID}); err != nil {
			return fmt.Errorf("delete role: %w", err)
		}
		return r.log(ctx, event{
			workspaceID: actor.WorkspaceID, actor: &actor.UserID, action: ActionRoleDeleted, target: roleID.String(),
			before: map[string]any{"name": role.Name, "permissions": role.Permissions},
			note:   fmt.Sprintf("%d members moved to %s", len(moved), target.Name),
		}, meta)
	})
}

// ---------- validation helpers ----------

func validateRole(in RoleInput, nameEditable bool) error {
	var details []apperr.Detail
	name := strings.TrimSpace(in.Name)
	if nameEditable && (name == "" || utf8.RuneCountInString(name) > 50) {
		details = append(details, apperr.Detail{Field: "name", Message: "Enter a role name of up to 50 characters"})
	}
	if utf8.RuneCountInString(strings.TrimSpace(in.Description)) > 200 {
		details = append(details, apperr.Detail{Field: "description", Message: "Keep the description under 200 characters"})
	}
	if !slices.Contains(colors, in.Color) {
		details = append(details, apperr.Detail{Field: "color", Message: "Choose one of the colors"})
	}
	if !slices.Contains(scopes, in.ChatScope) {
		details = append(details, apperr.Detail{Field: "chat_scope", Message: "Choose which chats this role can see"})
	}
	if len(details) > 0 {
		return ErrInvalidInput.WithDetails(details...)
	}
	return validatePermissions(in.Permissions)
}

func validatePermissions(keys []string) error {
	for _, k := range keys {
		if _, ok := authctx.PermissionByKey(k); !ok {
			return ErrInvalidInput.WithDetails(apperr.Detail{Field: "permissions", Message: "Unknown permission " + k})
		}
	}
	return nil
}

// canGive enforces "no privilege escalation": you can only give what you have.
func canGive(actor authctx.Access, keys []string) error {
	for _, k := range keys {
		p, ok := authctx.PermissionByKey(k)
		if ok && !actor.Can(p) {
			return ErrCannotGrant.WithDetails(apperr.Detail{Field: "permissions", Message: k})
		}
	}
	return nil
}

// diffPermissions lists the keys added and removed between two sets.
func diffPermissions(before, after []string) RoleChange {
	var c RoleChange
	for _, k := range after {
		if !slices.Contains(before, k) {
			c.Gains = append(c.Gains, k)
			if p, ok := authctx.PermissionByKey(k); ok && sensitive(p) {
				c.SensitiveGain = true
			}
		}
	}
	for _, k := range before {
		if !slices.Contains(after, k) {
			c.Losses = append(c.Losses, k)
		}
	}
	return c
}

func sensitive(p authctx.Permission) bool {
	switch p {
	case authctx.PermContactsExport, authctx.PermContactsDelete, authctx.PermIntegrationsAPIKeys, authctx.PermTeamRoles:
		return true
	default:
		return false
	}
}

func scopeRank(scope string) int { return slices.Index(scopes, scope) }

func scopeLabel(scope string) string {
	switch scope {
	case authctx.ScopeAll:
		return "All chats"
	case authctx.ScopeTeam:
		return "Their team's chats"
	default:
		return "Only assigned chats"
	}
}
