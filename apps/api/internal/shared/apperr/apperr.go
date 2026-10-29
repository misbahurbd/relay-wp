// Package apperr defines typed application errors. Services return these;
// platform/httpx maps them to HTTP responses. This package knows nothing about HTTP.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error independently of transport.
type Kind int

// Error kinds. KindInternal is the zero value so unknown errors are never exposed.
const (
	KindInternal Kind = iota
	KindValidation
	KindUnauthorized
	KindForbidden
	KindNotFound
	KindConflict
	KindRateLimited
	KindUnavailable
	KindBadRequest
	KindNotImplemented
)

// Detail describes one problem, usually tied to an input field.
type Detail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// Error is an application error safe to show to clients.
type Error struct {
	Kind    Kind
	Code    string // machine-readable, snake_case, e.g. "email_taken"
	Message string // human-readable, safe for clients
	Details []Detail
	Err     error // underlying cause; never shown to clients
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// WithDetails returns a copy of e with details attached.
func (e *Error) WithDetails(d ...Detail) *Error {
	c := *e
	c.Details = append(append([]Detail(nil), e.Details...), d...)
	return &c
}

// Wrap returns a copy of e that records cause as the underlying error.
func (e *Error) Wrap(cause error) *Error {
	c := *e
	c.Err = cause
	return &c
}

func newErr(k Kind, code, msg string) *Error { return &Error{Kind: k, Code: code, Message: msg} }

// Validation reports invalid input.
func Validation(code, msg string) *Error { return newErr(KindValidation, code, msg) }

// Unauthorized reports a missing or invalid identity.
func Unauthorized(code, msg string) *Error { return newErr(KindUnauthorized, code, msg) }

// Forbidden reports a known identity without permission.
func Forbidden(code, msg string) *Error { return newErr(KindForbidden, code, msg) }

// NotFound reports a missing resource.
func NotFound(code, msg string) *Error { return newErr(KindNotFound, code, msg) }

// Conflict reports a state conflict, such as a duplicate.
func Conflict(code, msg string) *Error { return newErr(KindConflict, code, msg) }

// RateLimited reports too many requests.
func RateLimited(code, msg string) *Error { return newErr(KindRateLimited, code, msg) }

// Unavailable reports a dependency that is temporarily down.
func Unavailable(code, msg string) *Error { return newErr(KindUnavailable, code, msg) }

// BadRequest reports a request that can't be read at all, such as a webhook whose signature fails.
func BadRequest(code, msg string) *Error { return newErr(KindBadRequest, code, msg) }

// NotImplemented reports an endpoint that exists but isn't built yet.
func NotImplemented(code, msg string) *Error { return newErr(KindNotImplemented, code, msg) }

// As returns the *Error in err's chain, if any.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// IsKind reports whether err is an *Error of kind k.
func IsKind(err error, k Kind) bool {
	e, ok := As(err)
	return ok && e.Kind == k
}
