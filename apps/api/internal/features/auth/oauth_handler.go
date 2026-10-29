package auth

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

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

func (h *Handler) oauthComplete(ctx context.Context, in *oauthCompleteInput) (*messageOutput, error) {
	if err := h.svc.SendOAuthEmailCode(ctx, in.Body.Token, in.Body.Email, in.Body.CodeVerifier); err != nil {
		return nil, err
	}
	return message202("We sent a 6-digit code to that address."), nil
}

func (h *Handler) oauthCompleteVerify(ctx context.Context, in *oauthCompleteVerifyInput) (*tokenOutput, error) {
	t, err := h.svc.CompleteOAuthSignup(ctx, in.Body.Token, in.Body.Code, in.Body.CodeVerifier,
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
