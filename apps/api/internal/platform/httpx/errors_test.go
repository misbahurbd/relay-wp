package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestAppErrorDecidesResponse(t *testing.T) {
	err := fmt.Errorf("wrap: %w", apperr.Conflict("email_taken", "Email already in use"))
	b := newErrorBody(http.StatusInternalServerError, "unexpected error occurred", []error{err})

	if b.GetStatus() != http.StatusConflict || b.Code != "email_taken" || b.Message != "Email already in use" {
		t.Errorf("got %d %q %q", b.GetStatus(), b.Code, b.Message)
	}
}

func TestUnknownErrorIsHidden(t *testing.T) {
	b := newErrorBody(http.StatusInternalServerError, "unexpected error occurred",
		[]error{errors.New("pq: password authentication failed")})

	if b.GetStatus() != http.StatusInternalServerError || b.Code != "internal_error" {
		t.Errorf("got %d %q", b.GetStatus(), b.Code)
	}
	if strings.Contains(b.Message, "password") || len(b.Details) != 0 {
		t.Errorf("internal error text leaked: %+v", b)
	}
}

func TestInternalAppErrorIsHidden(t *testing.T) {
	err := (&apperr.Error{Code: "db_down", Message: "secret detail"})
	b := newErrorBody(http.StatusInternalServerError, "", []error{err})

	if b.Code != "internal_error" || b.Message == "secret detail" {
		t.Errorf("internal app error leaked: %+v", b)
	}
}

func TestValidationDetailsKeepLocation(t *testing.T) {
	b := newErrorBody(http.StatusUnprocessableEntity, "validation failed",
		[]error{&huma.ErrorDetail{Location: "body.email", Message: "expected string"}})

	if b.Code != "validation_failed" || len(b.Details) != 1 || b.Details[0].Field != "body.email" {
		t.Errorf("got %+v", b)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, apperr.RateLimited("rate_limited", "Too many requests"))

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"rate_limited"`) {
		t.Errorf("body = %s", rec.Body.String())
	}
}
