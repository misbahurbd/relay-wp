package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
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

func (h *Handler) signup(ctx context.Context, in *signupInput) (*messageOutput, error) {
	err := h.svc.Signup(ctx, SignupInput{
		Email: in.Body.Email, Password: in.Body.Password,
		FirstName: in.Body.FirstName, LastName: in.Body.LastName, Timezone: in.Body.Timezone,
	}, meta(ctx, in.RequestHeaders, clientWeb))
	if err != nil {
		return nil, err
	}
	return message202("Check your inbox to verify your email address."), nil
}

func (h *Handler) verifyEmail(ctx context.Context, in *tokenInput) (*struct{}, error) {
	return nil, h.svc.VerifyEmail(ctx, in.Body.Token, meta(ctx, in.RequestHeaders, clientWeb))
}

func (h *Handler) resendVerification(ctx context.Context, in *emailInput) (*messageOutput, error) {
	if err := h.svc.ResendVerification(ctx, in.Body.Email, meta(ctx, in.RequestHeaders, clientWeb)); err != nil {
		return nil, err
	}
	return message202(checkInbox), nil
}

func (h *Handler) login(ctx context.Context, in *loginInput) (*tokenOutput, error) {
	t, err := h.svc.Login(ctx, in.Body.Email, in.Body.Password, meta(ctx, in.RequestHeaders, in.Body.Client))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

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

func (h *Handler) forgotPassword(ctx context.Context, in *emailInput) (*messageOutput, error) {
	if err := h.svc.ForgotPassword(ctx, in.Body.Email, meta(ctx, in.RequestHeaders, "")); err != nil {
		return nil, err
	}
	return message202(checkInbox), nil
}

func (h *Handler) resetPassword(ctx context.Context, in *resetPasswordInput) (*struct{}, error) {
	return nil, h.svc.ResetPassword(ctx, in.Body.Token, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) changePassword(ctx context.Context, in *changePasswordInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.ChangePassword(ctx, uid, sid, in.Body.CurrentPassword, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
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

// --- Social sign-in ---

func (h *Handler) listProviders(context.Context, *struct{}) (*providersOutput, error) {
	out := &providersOutput{}
	out.Body.Providers = h.svc.ProviderNames()
	return out, nil
}

func (h *Handler) oauthStart(ctx context.Context, in *oauthStartInput) (*redirectOutput, error) {
	url, state, err := h.svc.StartOAuth(ctx, OAuthStart{
		Provider: in.Provider, Intent: intentLogin, Client: in.Client, RedirectTo: in.RedirectTo,
		Timezone: in.Timezone, AppCodeChallenge: in.CodeChallenge,
	})
	if err != nil {
		return nil, err
	}
	return redirect(url, h.cookies.oauthState(state)), nil
}

func (h *Handler) oauthLink(ctx context.Context, in *oauthLinkInput) (*redirectOutput, error) {
	uid, sid := identity(ctx)
	url, state, err := h.svc.StartOAuth(ctx, OAuthStart{
		Provider: in.Provider, Intent: intentLink, LinkUserID: uid, SessionID: sid,
		Client: clientWeb, RedirectTo: in.RedirectTo,
	})
	if err != nil {
		return nil, err
	}
	return redirect(url, h.cookies.oauthState(state)), nil
}

func (h *Handler) oauthCallback(ctx context.Context, in *oauthCallbackInput) (*redirectOutput, error) {
	res := h.svc.CompleteOAuth(ctx, in.Provider, in.Code, in.State, in.StateCookie, in.Error,
		meta(ctx, in.RequestHeaders, clientWeb))
	cookies := []http.Cookie{h.cookies.clearOAuthState()}
	if res.Tokens != nil {
		cookies = append(cookies, h.cookies.set(*res.Tokens, time.Now())...)
	}
	return redirect(res.URL(), cookies...), nil
}

func (h *Handler) oauthComplete(ctx context.Context, in *oauthCompleteInput) (*tokenOutput, error) {
	t, err := h.svc.CompleteOAuthSignup(ctx, in.Body.Token, in.Body.Email, in.Body.CodeVerifier,
		meta(ctx, in.RequestHeaders, ""))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) oauthToken(ctx context.Context, in *oauthTokenInput) (*tokenOutput, error) {
	t, err := h.svc.RedeemAppCode(ctx, in.Body.Code, in.Body.CodeVerifier, meta(ctx, in.RequestHeaders, clientMobile))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) setPassword(ctx context.Context, in *setPasswordInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.SetPassword(ctx, uid, sid, in.Body.NewPassword, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) listIdentities(ctx context.Context, _ *authedInput) (*identitiesOutput, error) {
	uid, _ := identity(ctx)
	hasPassword, ids, err := h.svc.ListIdentities(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := &identitiesOutput{}
	out.Body.HasPassword = hasPassword
	out.Body.Identities = make([]identityDTO, 0, len(ids))
	for _, i := range ids {
		out.Body.Identities = append(out.Body.Identities, identityDTO(i))
	}
	return out, nil
}

func (h *Handler) unlinkIdentity(ctx context.Context, in *identityProviderInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.UnlinkIdentity(ctx, uid, sid, in.Provider, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) facebookDataDeletion(ctx context.Context, in *dataDeletionInput) (*dataDeletionOutput, error) {
	form, err := url.ParseQuery(string(in.RawBody))
	if err != nil {
		return nil, ErrInvalidSignedRequest
	}
	statusURL, code, err := h.svc.FacebookDataDeletion(ctx, form.Get("signed_request"), meta(ctx, in.RequestHeaders, ""))
	if err != nil {
		return nil, err
	}
	out := &dataDeletionOutput{}
	out.Body.URL, out.Body.ConfirmationCode = statusURL, code
	return out, nil
}

// redirect is a 302 that is never cached (it may carry one-time values).
func redirect(location string, cookies ...http.Cookie) *redirectOutput {
	return &redirectOutput{Status: http.StatusFound, Location: location, CacheControl: "no-store", SetCookie: cookies}
}
