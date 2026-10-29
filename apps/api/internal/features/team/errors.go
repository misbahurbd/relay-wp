package team

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the team service.
var (
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")

	ErrRoleNotFound   = apperr.NotFound("role_not_found", "Role not found")
	ErrMemberNotFound = apperr.NotFound("member_not_found", "Member not found")
	ErrTeamNotFound   = apperr.NotFound("team_not_found", "Team not found")
	ErrInviteNotFound = apperr.NotFound("invite_not_found", "This invite link isn't valid")
	ErrOverrideGone   = apperr.NotFound("override_not_found", "That permission was already removed")

	ErrRoleLocked  = apperr.Forbidden("role_locked", "The Owner role always has full access and can't be edited")
	ErrRoleChanged = apperr.Conflict("role_changed", "Someone else changed this role. Reload to see their changes.")
	ErrRoleName    = apperr.Conflict("role_name_taken", "Another role already has that name")
	ErrSystemRole  = apperr.Conflict("system_role", "System roles can't be deleted. Reset it to its defaults instead.")
	ErrNotSystem   = apperr.Conflict("not_system_role", "Only system roles have defaults to reset to")
	ErrTeamName    = apperr.Conflict("team_name_taken", "Another team already has that name")

	ErrCannotChangeSelf = apperr.Forbidden("cannot_change_self", "You can't change your own access. Ask another admin.")
	ErrCannotGrant      = apperr.Forbidden("cannot_grant", "You can only give permissions you have yourself")
	ErrOwnerProtected   = apperr.Forbidden("owner_protected", "Only the owner can change the owner's access")
	ErrOwnerRole        = apperr.Forbidden("owner_role", "The Owner role is given only by transferring ownership")
	ErrOwnerMustLeave   = apperr.Conflict("owner_must_transfer", "Transfer ownership before leaving the workspace")
	ErrConfirmName      = apperr.Validation("confirm_name_mismatch", "Type the member's name exactly to confirm")

	ErrNotEnoughSeats = apperr.Conflict("not_enough_seats",
		"There aren't enough seats. Remove someone, invite them as Viewers, or add seats.")
	ErrAlreadyMember = apperr.Conflict("already_member", "This person is already in the workspace")
	ErrInvitePending = apperr.Conflict("invite_pending", "This person already has a pending invite")
	ErrInviteExpired = apperr.Conflict("invite_expired", "This invite has expired. Ask for a new one.")
	ErrInviteRevoked = apperr.Conflict("invite_revoked", "This invite is no longer active")
	ErrInviteUsed    = apperr.Conflict("invite_accepted", "This invite has already been used")
	ErrInviteEmail   = apperr.Forbidden("invite_email_mismatch",
		"This invite is for a different email address. Sign in with the invited address.")
	ErrEmailNotVerified = apperr.Forbidden("email_not_verified", "Verify your email address before joining")
	ErrResendLimit      = apperr.RateLimited("resend_limit", "This invite has been sent 5 times. Revoke it and invite again.")
)
