package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const checkInbox = "If that address can receive email, a message is on its way."

// Handler turns HTTP requests into service calls. It holds no business rules.
type Handler struct {
	svc            *Service
	cookies        CookieConfig
	allowedOrigins []string
}

// NewHandler builds the auth HTTP handler. svc may be nil when only the OpenAPI spec is needed.
func NewHandler(svc *Service, cookies CookieConfig, allowedOrigins []string) *Handler {
	return &Handler{svc: svc, cookies: cookies, allowedOrigins: allowedOrigins}
}

func meta(ctx context.Context, h RequestHeaders, client string) RequestMeta {
	return RequestMeta{IP: middleware.ClientIP(ctx), UserAgent: h.UserAgent, Client: client, DeviceHeader: h.DeviceName}
}

// tokenResponse sets cookies for the web; mobile gets the tokens in the body.
func (h *Handler) tokenResponse(t Tokens) *tokenOutput {
	out := &tokenOutput{Body: tokenBody{AccessExpiresAt: t.AccessExpiresAt, RefreshExpiresAt: t.RefreshExpiresAt}}
	if t.Client == clientMobile {
		out.Body.AccessToken, out.Body.RefreshToken = t.AccessToken, t.RefreshToken
		return out
	}
	out.SetCookie = h.cookies.set(t, time.Now())
	return out
}

// identity is set by middleware.RequireAuth on every authenticated route.
func identity(ctx context.Context) (userID, sessionID uuid.UUID) {
	uid, _ := authctx.UserID(ctx)
	sid, _ := authctx.SessionID(ctx)
	return uid, sid
}

// redirect is a 302 that is never cached (it may carry one-time values).
func redirect(location string, cookies ...http.Cookie) *redirectOutput {
	return &redirectOutput{Status: http.StatusFound, Location: location, CacheControl: "no-store", SetCookie: cookies}
}
