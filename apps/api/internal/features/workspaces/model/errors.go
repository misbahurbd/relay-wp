package model

import "github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"

// Errors returned by the workspaces service.
var (
	// ErrNotFound is a workspace that doesn't exist or that the user can't see.
	ErrNotFound = apperr.NotFound("workspace_not_found", "Workspace not found")
	// ErrInvalidInput carries per-field details.
	ErrInvalidInput = apperr.Validation("invalid_input", "Some fields are invalid")
	// ErrEmailNotVerified blocks creating a workspace before the email is confirmed.
	ErrEmailNotVerified = apperr.Forbidden("email_not_verified", "Verify your email address before creating a workspace")
	// ErrLimitReached allows one trial workspace per user until it is activated.
	ErrLimitReached = apperr.Conflict("workspace_limit_reached",
		"You already have a workspace on a free trial. Finish setting it up first.")
	// ErrDeleteSubscription blocks deleting a workspace while its subscription is live (Q17).
	ErrDeleteSubscription = apperr.Conflict("workspace_has_subscription",
		"This workspace has an active subscription. Cancel it on the Billing page before deleting the workspace.")
	// ErrDeleteLifetime asks the owner to confirm giving up a lifetime deal before deleting (Q17).
	ErrDeleteLifetime = apperr.Conflict("workspace_has_lifetime_deal",
		"This workspace has a lifetime deal. Deleting the workspace gives up the deal for good. Confirm to delete anyway.")
)
