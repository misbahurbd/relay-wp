package handler

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/auth/service"
)

// --- Social sign-in ---

func (h *Handler) listProviders(context.Context, *struct{}) (*dto.ProvidersOutput, error) {
	out := &dto.ProvidersOutput{}
	out.Body.Providers = h.svc.ProviderNames()
	return out, nil
}

func (h *Handler) oauthStart(ctx context.Context, in *dto.OauthStartInput) (*dto.RedirectOutput, error) {
	url, state, err := h.svc.StartOAuth(ctx, model.OAuthStart{
		Provider: in.Provider, Intent: model.IntentLogin, Client: in.Client, RedirectTo: in.RedirectTo,
		Timezone: in.Timezone, AppCodeChallenge: in.CodeChallenge,
	})
	if err != nil {
		return nil, err
	}
	return redirect(url, h.cookies.oauthState(state)), nil
}

func (h *Handler) oauthLink(ctx context.Context, in *dto.OauthLinkInput) (*dto.RedirectOutput, error) {
	uid, sid := identity(ctx)
	url, state, err := h.svc.StartOAuth(ctx, model.OAuthStart{
		Provider: in.Provider, Intent: model.IntentLink, LinkUserID: uid, SessionID: sid,
		Client: model.ClientWeb, RedirectTo: in.RedirectTo,
	})
	if err != nil {
		return nil, err
	}
	return redirect(url, h.cookies.oauthState(state)), nil
}

func (h *Handler) oauthCallback(ctx context.Context, in *dto.OauthCallbackInput) (*dto.RedirectOutput, error) {
	res := h.svc.CompleteOAuth(ctx, in.Provider, in.Code, in.State, in.StateCookie, in.Error,
		meta(ctx, in.RequestHeaders, model.ClientWeb))
	cookies := []http.Cookie{h.cookies.clearOAuthState()}
	if res.Tokens != nil {
		cookies = append(cookies, h.cookies.set(*res.Tokens, time.Now())...)
	}
	return redirect(res.URL(), cookies...), nil
}

func (h *Handler) oauthComplete(ctx context.Context, in *dto.OauthCompleteInput) (*dto.MessageOutput, error) {
	if err := h.svc.SendOAuthEmailCode(ctx, in.Body.Token, in.Body.Email, in.Body.CodeVerifier); err != nil {
		return nil, err
	}
	return dto.Message202("We sent a 6-digit code to that address."), nil
}

func (h *Handler) oauthCompleteVerify(ctx context.Context, in *dto.OauthCompleteVerifyInput) (*dto.TokenOutput, error) {
	t, err := h.svc.CompleteOAuthSignup(ctx, in.Body.Token, in.Body.Code, in.Body.CodeVerifier,
		meta(ctx, in.RequestHeaders, ""))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) oauthToken(ctx context.Context, in *dto.OauthTokenInput) (*dto.TokenOutput, error) {
	t, err := h.svc.RedeemAppCode(ctx, in.Body.Code, in.Body.CodeVerifier, meta(ctx, in.RequestHeaders, model.ClientMobile))
	if err != nil {
		return nil, err
	}
	return h.tokenResponse(t), nil
}

func (h *Handler) listIdentities(ctx context.Context, _ *dto.AuthedInput) (*dto.IdentitiesOutput, error) {
	uid, _ := identity(ctx)
	hasPassword, ids, err := h.svc.ListIdentities(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := &dto.IdentitiesOutput{}
	out.Body.HasPassword = hasPassword
	out.Body.Identities = make([]dto.IdentityDTO, 0, len(ids))
	for _, i := range ids {
		out.Body.Identities = append(out.Body.Identities, dto.IdentityDTO(i))
	}
	return out, nil
}

func (h *Handler) unlinkIdentity(ctx context.Context, in *dto.IdentityProviderInput) (*struct{}, error) {
	uid, sid := identity(ctx)
	return nil, h.svc.UnlinkIdentity(ctx, uid, sid, in.Provider, meta(ctx, in.RequestHeaders, ""))
}

func (h *Handler) facebookDataDeletion(ctx context.Context, in *dto.DataDeletionInput) (*dto.DataDeletionOutput, error) {
	form, err := url.ParseQuery(string(in.RawBody))
	if err != nil {
		return nil, service.ErrInvalidSignedRequest
	}
	statusURL, code, err := h.svc.FacebookDataDeletion(ctx, form.Get("signed_request"), meta(ctx, in.RequestHeaders, ""))
	if err != nil {
		return nil, err
	}
	out := &dto.DataDeletionOutput{}
	out.Body.URL, out.Body.ConfirmationCode = statusURL, code
	return out, nil
}
