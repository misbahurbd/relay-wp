package auth

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func (h *Handler) refresh(ctx context.Context, in *refreshInput) (*tokenOutput, error) {
	token, client := "", clientWeb
	switch {
	case in.Body != nil && in.Body.RefreshToken != "":
		token, client = in.Body.RefreshToken, clientMobile
	case in.Cookie != "":
		// Cookies are sent automatically by browsers, so check where the request came from.
		if !middleware.SameOrigin("POST", in.Origin, in.Referer, h.allowedOrigins) {
			return nil, apperr.Forbidden("invalid_origin", "Request origin is not allowed")
		}
		token = in.Cookie
	}
	t, err := h.svc.Refresh(ctx, token, meta(ctx, in.RequestHeaders, client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) logout(ctx context.Context, in *authedInput) (*cookieOutput, error) {
	uid, sid := identity(ctx)
	if err := h.svc.Logout(ctx, uid, sid, meta(ctx, in.RequestHeaders, "")); err != nil {
		return nil, err
	}
	return &cookieOutput{SetCookie: h.cookies.clear()}, nil
}

func (h *Handler) logoutAll(ctx context.Context, in *authedInput) (*cookieOutput, error) {
	uid, _ := identity(ctx)
	if err := h.svc.LogoutAll(ctx, uid, meta(ctx, in.RequestHeaders, "")); err != nil {
		return nil, err
	}
	return &cookieOutput{SetCookie: h.cookies.clear()}, nil
}

func (h *Handler) listSessions(ctx context.Context, _ *authedInput) (*sessionsOutput, error) {
	uid, sid := identity(ctx)
	sessions, err := h.svc.ListSessions(ctx, uid, sid)
	if err != nil {
		return nil, err
	}
	out := &sessionsOutput{}
	out.Body.Sessions = make([]sessionDTO, 0, len(sessions))
	for _, s := range sessions {
		out.Body.Sessions = append(out.Body.Sessions, sessionDTO{
			ID: s.ID, DeviceName: s.DeviceName, DeviceType: s.DeviceType, Provider: s.Provider, Client: s.Client,
			LastIP: s.LastIP, LastUsedAt: s.LastUsedAt, CreatedAt: s.CreatedAt, Current: s.Current,
		})
	}
	return out, nil
}

func (h *Handler) revokeSession(ctx context.Context, in *sessionIDInput) (*struct{}, error) {
	uid, _ := identity(ctx)
	return nil, h.svc.RevokeSession(ctx, uid, in.ID, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) listEvents(ctx context.Context, _ *authedInput) (*eventsOutput, error) {
	uid, _ := identity(ctx)
	events, err := h.svc.ListEvents(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := &eventsOutput{}
	out.Body.Events = make([]eventDTO, 0, len(events))
	for _, e := range events {
		out.Body.Events = append(out.Body.Events, eventDTO(e))
	}
	return out, nil
}
