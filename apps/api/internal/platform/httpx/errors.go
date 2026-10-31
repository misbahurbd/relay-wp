// Package httpx holds HTTP infrastructure: the server, and the single place
// where application errors become HTTP responses.
package httpx

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// ErrorBody is the JSON shape of every error response.
type ErrorBody struct {
	status  int
	Code    string          `json:"code" example:"not_found" doc:"Machine-readable error code"`
	Message string          `json:"message" example:"Resource not found" doc:"Human-readable message"`
	Details []apperr.Detail `json:"details,omitempty" doc:"Per-field problems, if any"`
}

// Error implements error.
func (e *ErrorBody) Error() string { return e.Code + ": " + e.Message }

// GetStatus implements huma.StatusError.
func (e *ErrorBody) GetStatus() int { return e.status }

// InstallErrorHandler makes huma build every error response as an ErrorBody.
// huma exposes this hook only as package-level variables, so call it once at startup.
func InstallErrorHandler() {
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		return newErrorBody(status, msg, errs)
	}
	huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
		body := newErrorBody(status, msg, errs)
		if body.status >= http.StatusInternalServerError && ctx != nil {
			logger.FromContext(ctx.Context()).Error("request failed",
				"status", body.status, "err", errors.Join(errs...))
		}
		return body
	}
}

func newErrorBody(status int, msg string, errs []error) *ErrorBody {
	// An application error anywhere in errs decides the response.
	for _, err := range errs {
		if ae, ok := apperr.As(err); ok {
			s := statusFor(ae.Kind)
			body := &ErrorBody{status: s, Code: ae.Code, Message: ae.Message, Details: ae.Details}
			if s >= http.StatusInternalServerError && ae.Kind == apperr.KindInternal {
				body.Code, body.Message, body.Details = "internal_error", "Something went wrong", nil
			}
			return body
		}
	}

	// Errors raised by huma itself (validation, parsing) or unknown errors.
	body := &ErrorBody{status: status, Code: codeFor(status), Message: msg}
	if status >= http.StatusInternalServerError {
		// Never leak internal error text.
		body.Message = "Something went wrong"
		return body
	}
	for _, err := range errs {
		if err == nil {
			continue
		}
		var d huma.ErrorDetailer
		if errors.As(err, &d) {
			ed := d.ErrorDetail()
			body.Details = append(body.Details, apperr.Detail{Field: ed.Location, Message: ed.Message})
			continue
		}
		body.Details = append(body.Details, apperr.Detail{Message: err.Error()})
	}
	return body
}

func statusFor(k apperr.Kind) int {
	switch k {
	case apperr.KindValidation:
		return http.StatusUnprocessableEntity
	case apperr.KindUnauthorized:
		return http.StatusUnauthorized
	case apperr.KindForbidden:
		return http.StatusForbidden
	case apperr.KindNotFound:
		return http.StatusNotFound
	case apperr.KindConflict:
		return http.StatusConflict
	case apperr.KindRateLimited:
		return http.StatusTooManyRequests
	case apperr.KindUnavailable:
		return http.StatusServiceUnavailable
	case apperr.KindBadRequest:
		return http.StatusBadRequest
	case apperr.KindNotImplemented:
		return http.StatusNotImplemented
	case apperr.KindPaymentRequired:
		return http.StatusPaymentRequired
	default:
		return http.StatusInternalServerError
	}
}

func codeFor(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusUnprocessableEntity:
		return "validation_failed"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusServiceUnavailable:
		return "unavailable"
	case http.StatusNotImplemented:
		return "not_implemented"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "error"
	}
}

// WriteError writes an ErrorBody from plain net/http code (middleware, not-found handlers).
func WriteError(w http.ResponseWriter, err error) {
	write(w, newErrorBody(http.StatusInternalServerError, "", []error{err}))
}

// WriteStatus writes an ErrorBody for a bare HTTP status, such as 405.
func WriteStatus(w http.ResponseWriter, status int, msg string) {
	write(w, newErrorBody(status, msg, nil))
}

func write(w http.ResponseWriter, body *ErrorBody) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(body.status)
	_ = json.NewEncoder(w).Encode(body)
}
