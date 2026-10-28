package apperr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestAsFindsWrappedError(t *testing.T) {
	base := apperr.NotFound("user_not_found", "User not found")
	err := fmt.Errorf("lookup: %w", base)

	got, ok := apperr.As(err)
	if !ok || got.Code != "user_not_found" {
		t.Fatalf("As() = %v, %v", got, ok)
	}
	if !apperr.IsKind(err, apperr.KindNotFound) {
		t.Error("IsKind should match NotFound")
	}
}

func TestWrapKeepsCauseAndDoesNotMutate(t *testing.T) {
	base := apperr.Conflict("email_taken", "Email already in use")
	cause := errors.New("unique violation")
	wrapped := base.Wrap(cause)

	if !errors.Is(wrapped, cause) {
		t.Error("wrapped error should unwrap to cause")
	}
	if base.Err != nil {
		t.Error("Wrap mutated the original error")
	}
}

func TestWithDetailsDoesNotMutate(t *testing.T) {
	base := apperr.Validation("invalid_input", "Invalid input")
	withD := base.WithDetails(apperr.Detail{Field: "email", Message: "required"})

	if len(base.Details) != 0 || len(withD.Details) != 1 {
		t.Errorf("base=%v withD=%v", base.Details, withD.Details)
	}
}
