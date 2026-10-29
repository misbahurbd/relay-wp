package middleware

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// StaffChecker reports whether a signed-in user may use the admin tools.
type StaffChecker interface {
	IsStaff(ctx context.Context, userID uuid.UUID) (bool, error)
}

var (
	errStaffOnly  = apperr.Forbidden("staff_only", "Only Relay staff can do that")
	errStaffCheck = apperr.Unavailable("staff_check_failed", "Could not check your access. Try again.")
)

// RequireStaff is a huma middleware for routes under /v1/admin. It runs after
// RequireAuth; a nil checker refuses everyone.
func RequireStaff(api huma.API, s StaffChecker) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		userID, signedIn := authctx.UserID(ctx.Context())
		if !signedIn || s == nil {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errStaffOnly)
			return
		}
		ok, err := s.IsStaff(ctx.Context(), userID)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusServiceUnavailable, "", errStaffCheck.Wrap(err))
			return
		}
		if !ok {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "", errStaffOnly)
			return
		}
		next(ctx)
	}
}
