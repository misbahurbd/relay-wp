package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

// MaxMediaBytes is the largest file FetchMedia downloads (WhatsApp's document limit).
const MaxMediaBytes = 100 << 20

// mediaTimeout bounds a media upload or download; the shared client's default is
// too short for large files. The transport is still shared.
const mediaTimeout = 5 * time.Minute

// sendMessage posts one Cloud API message from phoneNumberID to to and returns its wamid.
// msg holds the type-specific fields.
func (g *Graph) sendMessage(ctx context.Context, token, phoneNumberID, to string, msg map[string]any) (string, error) {
	body := map[string]any{
		"messaging_product": "whatsapp",
		"recipient_type":    "individual",
		"to":                model.Digits(to),
	}
	maps.Copy(body, msg)
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

// withContext quotes replyTo (a wamid) when set.
func withContext(msg map[string]any, replyTo string) map[string]any {
	if replyTo != "" {
		msg["context"] = map[string]string{"message_id": replyTo}
	}
	return msg
}

// SendText sends a free-form text message; replyTo quotes an earlier message.
func (g *Graph) SendText(ctx context.Context, token, phoneNumberID, to, body, replyTo string) (string, error) {
	return g.sendMessage(ctx, token, phoneNumberID, to, withContext(map[string]any{
		"type": "text",
		"text": map[string]any{"body": body, "preview_url": true},
	}, replyTo))
}

// SendMedia sends a file uploaded to Meta by its media ID.
func (g *Graph) SendMedia(ctx context.Context, token, phoneNumberID, to string, m model.OutboundMedia, replyTo string) (string, error) {
	switch m.Kind {
	case "image", "video", "audio", "document", "sticker":
	default:
		return "", fmt.Errorf("graph: unsupported media kind %q", m.Kind)
	}
	media := map[string]any{"id": m.MediaID}
	if m.Caption != "" && m.Kind != "audio" && m.Kind != "sticker" {
		media["caption"] = m.Caption
	}
	if m.Filename != "" && m.Kind == "document" {
		media["filename"] = m.Filename
	}
	return g.sendMessage(ctx, token, phoneNumberID, to, withContext(map[string]any{"type": m.Kind, m.Kind: media}, replyTo))
}

// SendTemplateMessage sends an approved template with its parameters.
func (g *Graph) SendTemplateMessage(ctx context.Context, token, phoneNumberID, to string, t model.OutboundTemplate) (string, error) {
	tpl := map[string]any{"name": t.Name, "language": map[string]string{"code": t.Language}}
	if c := templateParams(t); len(c) > 0 {
		tpl["components"] = c
	}
	return g.sendMessage(ctx, token, phoneNumberID, to, map[string]any{"type": "template", "template": tpl})
}

// templateParams builds the send-time components: header, body and URL button
// parameters. Components without parameters are left out.
func templateParams(t model.OutboundTemplate) []map[string]any {
	var out []map[string]any
	switch {
	case t.HeaderText != "":
		out = append(out, map[string]any{"type": "header", "parameters": []map[string]any{{"type": "text", "text": t.HeaderText}}})
	case t.HeaderMediaID != "" && (t.HeaderKind == "image" || t.HeaderKind == "video" || t.HeaderKind == "document"):
		out = append(out, map[string]any{"type": "header", "parameters": []map[string]any{
			{"type": t.HeaderKind, t.HeaderKind: map[string]string{"id": t.HeaderMediaID}},
		}})
	}
	if len(t.BodyParams) > 0 {
		params := make([]map[string]any, 0, len(t.BodyParams))
		for _, p := range t.BodyParams {
			params = append(params, map[string]any{"type": "text", "text": p})
		}
		out = append(out, map[string]any{"type": "body", "parameters": params})
	}
	for _, i := range slices.Sorted(maps.Keys(t.URLButtonParams)) {
		out = append(out, map[string]any{
			"type": "button", "sub_type": "url", "index": strconv.Itoa(i),
			"parameters": []map[string]any{{"type": "text", "text": t.URLButtonParams[i]}},
		})
	}
	return out
}

// SendReaction reacts to targetWAMID; an empty emoji removes the reaction.
func (g *Graph) SendReaction(ctx context.Context, token, phoneNumberID, to, targetWAMID, emoji string) (string, error) {
	return g.sendMessage(ctx, token, phoneNumberID, to, map[string]any{
		"type":     "reaction",
		"reaction": map[string]string{"message_id": targetWAMID, "emoji": emoji},
	})
}

// MarkRead marks an inbound message (and the ones before it) read.
func (g *Graph) MarkRead(ctx context.Context, token, phoneNumberID, wamid string) error {
	return g.call(ctx, http.MethodPost, "/"+url.PathEscape(phoneNumberID)+"/messages", token, nil, map[string]string{
		"messaging_product": "whatsapp", "status": "read", "message_id": wamid,
	}, nil)
}

var quoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// UploadMedia streams a file to /{phone_number_id}/media as multipart form data
// and returns Meta's media ID. size is the file's length (or < 0 if unknown).
func (g *Graph) UploadMedia(ctx context.Context, token, phoneNumberID, contentType, filename string, data io.Reader, size int64) (string, error) {
	if filename == "" {
		filename = "file"
	}
	// The form's head and tail are built up front so the file streams between
	// them, and the request length is known when size is.
	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	if err := mw.WriteField("messaging_product", "whatsapp"); err != nil {
		return "", fmt.Errorf("graph upload form: %w", err)
	}
	if err := mw.WriteField("type", contentType); err != nil {
		return "", fmt.Errorf("graph upload form: %w", err)
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+quoteEscaper.Replace(filename)+`"`)
	h.Set("Content-Type", contentType)
	if _, err := mw.CreatePart(h); err != nil {
		return "", fmt.Errorf("graph upload form: %w", err)
	}
	headLen := form.Len()
	if err := mw.Close(); err != nil {
		return "", fmt.Errorf("graph upload form: %w", err)
	}
	head, tail := form.Bytes()[:headLen], form.Bytes()[headLen:]

	q := url.Values{"appsecret_proof": {g.proof(token)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		g.base+"/"+url.PathEscape(phoneNumberID)+"/media?"+q.Encode(),
		io.MultiReader(bytes.NewReader(head), data, bytes.NewReader(tail)))
	if err != nil {
		return "", fmt.Errorf("graph upload request: %w", err)
	}
	req.ContentLength = -1
	if size >= 0 {
		req.ContentLength = int64(len(head)) + size + int64(len(tail))
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	client := *g.http
	client.Timeout = mediaTimeout
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("graph media upload: %w", stripURL(err))
	}
	defer func() { _ = res.Body.Close() }()
	var out struct {
		ID string `json:"id"`
	}
	if err := decodeGraph(res, "media", &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("graph: no media id in upload response")
	}
	return out.ID, nil
}

// FetchMedia looks up a media ID and downloads the file. Meta's download URL
// is short-lived and is never logged. The caller reads and closes body; it
// fails with an error once more than MaxMediaBytes have been read.
func (g *Graph) FetchMedia(ctx context.Context, token, mediaID string) (body io.ReadCloser, contentType string, size int64, err error) {
	var meta struct {
		URL      string `json:"url"`
		MimeType string `json:"mime_type"`
		FileSize int64  `json:"file_size"`
	}
	if err := g.call(ctx, http.MethodGet, "/"+url.PathEscape(mediaID), token, nil, nil, &meta); err != nil {
		return nil, "", 0, err
	}
	if meta.URL == "" {
		return nil, "", 0, errors.New("graph: no url for media")
	}
	if meta.FileSize > MaxMediaBytes {
		return nil, "", 0, model.ErrMediaTooLarge
	}
	u, err := url.Parse(meta.URL)
	if err != nil || (u.Scheme != "https" && u.Host != g.host()) {
		return nil, "", 0, errors.New("graph: unexpected media url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, meta.URL, nil)
	if err != nil {
		return nil, "", 0, errors.New("graph: bad media url")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := *g.http
	client.Timeout = mediaTimeout
	res, err := client.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("graph media download: %w", stripURL(err))
	}
	if res.StatusCode >= 300 {
		defer func() { _ = res.Body.Close() }()
		return nil, "", 0, decodeGraph(res, "media download", nil)
	}
	if res.ContentLength > MaxMediaBytes {
		_ = res.Body.Close()
		return nil, "", 0, model.ErrMediaTooLarge
	}
	contentType, size = meta.MimeType, meta.FileSize
	if contentType == "" {
		contentType = res.Header.Get("Content-Type")
	}
	if size == 0 && res.ContentLength > 0 {
		size = res.ContentLength
	}
	return &cappedBody{r: io.LimitReader(res.Body, MaxMediaBytes+1), c: res.Body}, contentType, size, nil
}

// host is the Graph base URL's host (tests serve media from it over http).
func (g *Graph) host() string {
	u, err := url.Parse(g.base)
	if err != nil {
		return ""
	}
	return u.Host
}

// cappedBody fails a download that grows past MaxMediaBytes instead of
// silently truncating it.
type cappedBody struct {
	r    io.Reader
	c    io.Closer
	read int64
}

func (b *cappedBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	b.read += int64(n)
	if b.read > MaxMediaBytes {
		return n, model.ErrMediaTooLarge
	}
	return n, err //nolint:wrapcheck // io.EOF must reach the caller as is
}

func (b *cappedBody) Close() error { return b.c.Close() } //nolint:wrapcheck // a plain io.Closer

// sendFailure is what a Cloud API refusal looks like to the inbox.
func sendFailure(g *GraphError, permanent bool) *model.SendError {
	title := g.Message
	// Cloud API messages start with "(#131047) ".
	if strings.HasPrefix(title, "(#") {
		if _, rest, ok := strings.Cut(title, ") "); ok {
			title = rest
		}
	}
	if title == "" {
		title = "Meta error " + strconv.Itoa(g.Code)
	}
	msg := g.ErrorData.Details
	if msg == "" {
		msg = g.UserMessage
	}
	return &model.SendError{Code: g.Code, Title: title, Message: msg, Permanent: permanent}
}

// retryableCodes are Cloud API errors that may succeed later: something went
// wrong, service unavailable, and rate limits.
var retryableCodes = map[int]bool{
	1: true, 2: true, 4: true, 80007: true,
	130429: true, 131000: true, 131016: true, 131056: true, 133004: true,
}

// classifySend maps a Graph error to what the inbox acts on. Meta's message
// errors become *model.SendError (permanent unless retryable); server and
// transport failures stay plain errors so the job retries.
func classifySend(err error) error {
	g, ok := asGraphError(err)
	if !ok {
		return err
	}
	switch {
	case g.authFailed():
		return sendFailure(g, true)
	case retryableCodes[g.Code]:
		return sendFailure(g, false)
	case g.Status >= 500 || g.Status == http.StatusTooManyRequests || g.Code == 0:
		return err
	default:
		return sendFailure(g, true)
	}
}
