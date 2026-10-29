package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// GraphBaseURL is Meta's Graph API host.
const GraphBaseURL = "https://graph.facebook.com"

// Graph is a small client for the Meta Graph API calls this feature makes. One
// instance serves every workspace: the business token is passed per call.
type Graph struct {
	http      *http.Client
	base      string
	appID     string
	appSecret string
}

// NewGraph builds the client. base is GraphBaseURL plus the pinned version, e.g.
// https://graph.facebook.com/v24.0; tests point it at a fake server.
func NewGraph(client *http.Client, base, appID, appSecret string) *Graph {
	return &Graph{http: client, base: strings.TrimRight(base, "/"), appID: appID, appSecret: appSecret}
}

// GraphError is an error answer from Meta.
type GraphError struct {
	Status  int    `json:"-"`
	Code    int    `json:"code"`
	Subcode int    `json:"error_subcode"`
	Type    string `json:"type"`
	Message string `json:"message"`
	// UserMessage is Meta's explanation for people, e.g. why a template was refused.
	UserMessage string `json:"error_user_msg"`
}

func (e *GraphError) Error() string {
	return fmt.Sprintf("graph: %d (code %d/%d): %s", e.Status, e.Code, e.Subcode, e.Message)
}

// authFailed means the token was revoked or lost a permission: the account is disconnected.
func (e *GraphError) authFailed() bool { return e.Code == 190 || e.Code == 10 }

func asGraphError(err error) (*GraphError, bool) {
	var g *GraphError
	ok := errors.As(err, &g)
	return g, ok
}

func isAuthFailure(err error) bool {
	g, ok := asGraphError(err)
	return ok && g.authFailed()
}

func (g *Graph) call(ctx context.Context, method, path, token string, query url.Values, body, out any) error {
	if query == nil {
		query = url.Values{}
	}
	if token != "" {
		// appsecret_proof ties the call to our app, so a leaked token alone is less useful.
		mac := hmac.New(sha256.New, []byte(g.appSecret))
		mac.Write([]byte(token))
		query.Set("appsecret_proof", hex.EncodeToString(mac.Sum(nil)))
	}
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("graph request: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, g.base+path+"?"+query.Encode(), reader)
	if err != nil {
		return fmt.Errorf("graph request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("graph %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	return decodeGraph(res, path, out)
}

// decodeGraph reads a Graph answer into out, or returns Meta's error. The caller closes the body.
func decodeGraph(res *http.Response, path string, out any) error {
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("graph read: %w", err)
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error GraphError `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		e.Error.Status = res.StatusCode
		return &e.Error
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("graph decode %s: %w", path, err)
		}
	}
	return nil
}

// ExchangeCode swaps the Embedded Signup code for a business integration token.
func (g *Graph) ExchangeCode(ctx context.Context, code string) (string, error) {
	var out struct {
		AccessToken string `json:"access_token"`
	}
	q := url.Values{"client_id": {g.appID}, "client_secret": {g.appSecret}, "code": {code}}
	if err := g.call(ctx, http.MethodGet, "/oauth/access_token", "", q, nil, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("graph: no access token in code exchange")
	}
	return out.AccessToken, nil
}

// WABAInfo is what this feature reads about a WhatsApp Business account.
type WABAInfo struct {
	Name          string `json:"name"`
	OwnerBusiness struct {
		ID string `json:"id"`
	} `json:"owner_business_info"`
}

// WABA reads a WhatsApp Business account; it fails if the token can't see it.
func (g *Graph) WABA(ctx context.Context, token, wabaID string) (WABAInfo, error) {
	var out WABAInfo
	err := g.call(ctx, http.MethodGet, "/"+url.PathEscape(wabaID), token,
		url.Values{"fields": {"name,owner_business_info"}}, nil, &out)
	return out, err
}

// PhoneInfo is what this feature reads about a phone number.
type PhoneInfo struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name"`
	NameStatus         string `json:"name_status"`
	QualityRating      string `json:"quality_rating"`
	MessagingLimitTier string `json:"messaging_limit_tier"`
}

// PhoneNumber reads a number's display details, name review and quality.
func (g *Graph) PhoneNumber(ctx context.Context, token, phoneNumberID string) (PhoneInfo, error) {
	var out PhoneInfo
	err := g.call(ctx, http.MethodGet, "/"+url.PathEscape(phoneNumberID), token,
		url.Values{"fields": {"display_phone_number,verified_name,name_status,quality_rating,messaging_limit_tier"}}, nil, &out)
	return out, err
}

// Register turns on the Cloud API for a number, setting its two-step verification PIN.
func (g *Graph) Register(ctx context.Context, token, phoneNumberID, pin string) error {
	return g.call(ctx, http.MethodPost, "/"+url.PathEscape(phoneNumberID)+"/register", token, nil,
		map[string]string{"messaging_product": "whatsapp", "pin": pin}, nil)
}

// SubscribeApp sends the WABA's webhooks to this app.
func (g *Graph) SubscribeApp(ctx context.Context, token, wabaID string) error {
	return g.call(ctx, http.MethodPost, "/"+url.PathEscape(wabaID)+"/subscribed_apps", token, nil, nil, nil)
}

// CountTemplates returns how many message templates the WABA has (first page).
func (g *Graph) CountTemplates(ctx context.Context, token, wabaID string) (int, error) {
	var out struct {
		Data []json.RawMessage `json:"data"`
	}
	err := g.call(ctx, http.MethodGet, "/"+url.PathEscape(wabaID)+"/message_templates", token,
		url.Values{"fields": {"name"}, "limit": {"100"}}, nil, &out)
	return len(out.Data), err
}

// SendTemplate sends a template message and returns its WhatsApp message ID (wamid).
func (g *Graph) SendTemplate(ctx context.Context, token, phoneNumberID, to, name, lang string) (string, error) {
	body := map[string]any{
		"messaging_product": "whatsapp",
		"to":                to,
		"type":              "template",
		"template":          map[string]any{"name": name, "language": map[string]string{"code": lang}},
	}
	var out struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := g.call(ctx, http.MethodPost, "/"+url.PathEscape(phoneNumberID)+"/messages", token, nil, body, &out); err != nil {
		return "", err
	}
	if len(out.Messages) == 0 || out.Messages[0].ID == "" {
		return "", errors.New("graph: no message id in send response")
	}
	return out.Messages[0].ID, nil
}

// nameStatus maps Meta's display-name review states onto ours.
func nameStatus(meta string) string {
	switch strings.ToUpper(meta) {
	case "APPROVED", "AVAILABLE_WITHOUT_REVIEW":
		return "approved"
	case "DECLINED", "REJECTED", "EXPIRED":
		return "declined"
	default:
		return "pending_review"
	}
}

// qualityRating maps GREEN/YELLOW/RED; anything else is unknown (nil).
func qualityRating(meta string) *string {
	switch q := strings.ToLower(meta); q {
	case "green", "yellow", "red":
		return &q
	default:
		return nil
	}
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
