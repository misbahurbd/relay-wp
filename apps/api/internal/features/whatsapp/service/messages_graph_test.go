package service

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

// sendServer is a fake Graph that records one /messages call and answers with a wamid.
type sendServer struct {
	path, auth, proof string
	body              map[string]any
}

func newSendServer(t *testing.T) (*Graph, *sendServer) {
	t.Helper()
	rec := &sendServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.path, rec.auth, rec.proof = r.URL.Path, r.Header.Get("Authorization"), r.URL.Query().Get("appsecret_proof")
		_ = json.NewDecoder(r.Body).Decode(&rec.body)
		_, _ = w.Write([]byte(`{"messaging_product":"whatsapp","contacts":[{"input":"14155550142","wa_id":"14155550142"}],"messages":[{"id":"wamid.OUT1"}]}`))
	}))
	t.Cleanup(srv.Close)
	return NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret"), rec
}

func assertJSON(t *testing.T, got any, want string) {
	t.Helper()
	var wantAny any
	if err := json.Unmarshal([]byte(want), &wantAny); err != nil {
		t.Fatal(err)
	}
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(wantAny)
	if string(g) != string(w) {
		t.Fatalf("payload\n got %s\nwant %s", g, w)
	}
}

func TestSendMessagePayloads(t *testing.T) {
	tests := []struct {
		name string
		send func(g *Graph) (string, error)
		want string
	}{
		{
			"text with context",
			func(g *Graph) (string, error) {
				return g.SendText(t.Context(), "tok", "1001", "+14155550142", "Hi there", "wamid.IN1")
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"text",
			  "text":{"body":"Hi there","preview_url":true},"context":{"message_id":"wamid.IN1"}}`,
		},
		{
			"document with filename",
			func(g *Graph) (string, error) {
				return g.SendMedia(t.Context(), "tok", "1001", "+14155550142",
					model.OutboundMedia{Kind: "document", MediaID: "m1", Caption: "Your invoice", Filename: "invoice.pdf"}, "")
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"document",
			  "document":{"id":"m1","caption":"Your invoice","filename":"invoice.pdf"}}`,
		},
		{
			"audio drops caption and filename",
			func(g *Graph) (string, error) {
				return g.SendMedia(t.Context(), "tok", "1001", "14155550142",
					model.OutboundMedia{Kind: "audio", MediaID: "m2", Caption: "x", Filename: "a.ogg"}, "")
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"audio","audio":{"id":"m2"}}`,
		},
		{
			"template with header text, body params and url button",
			func(g *Graph) (string, error) {
				return g.SendTemplateMessage(t.Context(), "tok", "1001", "+14155550142", model.OutboundTemplate{
					Name: "order_update", Language: "en_US", HeaderText: "#1042",
					BodyParams: []string{"Daniel", "Friday"}, URLButtonParams: map[int]string{1: "1042"},
				})
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"template",
			  "template":{"name":"order_update","language":{"code":"en_US"},"components":[
			    {"type":"header","parameters":[{"type":"text","text":"#1042"}]},
			    {"type":"body","parameters":[{"type":"text","text":"Daniel"},{"type":"text","text":"Friday"}]},
			    {"type":"button","sub_type":"url","index":"1","parameters":[{"type":"text","text":"1042"}]}
			  ]}}`,
		},
		{
			"template with image header",
			func(g *Graph) (string, error) {
				return g.SendTemplateMessage(t.Context(), "tok", "1001", "+14155550142", model.OutboundTemplate{
					Name: "promo", Language: "en", HeaderKind: "image", HeaderMediaID: "m3",
				})
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"template",
			  "template":{"name":"promo","language":{"code":"en"},"components":[
			    {"type":"header","parameters":[{"type":"image","image":{"id":"m3"}}]}
			  ]}}`,
		},
		{
			"plain template (test message) has no components",
			func(g *Graph) (string, error) {
				return g.SendTemplate(t.Context(), "tok", "1001", "14155550142", "hello_world", "en_US")
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"template",
			  "template":{"name":"hello_world","language":{"code":"en_US"}}}`,
		},
		{
			"reaction",
			func(g *Graph) (string, error) {
				return g.SendReaction(t.Context(), "tok", "1001", "+14155550142", "wamid.IN1", "👍")
			},
			`{"messaging_product":"whatsapp","recipient_type":"individual","to":"14155550142","type":"reaction",
			  "reaction":{"message_id":"wamid.IN1","emoji":"👍"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, rec := newSendServer(t)
			wamid, err := tt.send(g)
			if err != nil {
				t.Fatal(err)
			}
			if wamid != "wamid.OUT1" {
				t.Fatalf("wamid %q", wamid)
			}
			if rec.path != "/v24.0/1001/messages" || rec.auth != "Bearer tok" || rec.proof != g.proof("tok") {
				t.Fatalf("path %q auth %q proof %q", rec.path, rec.auth, rec.proof)
			}
			assertJSON(t, rec.body, tt.want)
		})
	}
}

func TestMarkReadPayload(t *testing.T) {
	g, rec := newSendServer(t)
	if err := g.MarkRead(t.Context(), "tok", "1001", "wamid.IN1"); err != nil {
		t.Fatal(err)
	}
	if rec.path != "/v24.0/1001/messages" {
		t.Fatalf("path %q", rec.path)
	}
	assertJSON(t, rec.body, `{"messaging_product":"whatsapp","status":"read","message_id":"wamid.IN1"}`)
}

func TestUploadMedia(t *testing.T) {
	var path, auth, proof string
	var fields map[string]string
	var fileName, fileType, fileBody string
	var contentLength int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth, proof, contentLength = r.URL.Path, r.Header.Get("Authorization"), r.URL.Query().Get("appsecret_proof"), r.ContentLength
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		fields = map[string]string{}
		for {
			p, err := mr.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			b, _ := io.ReadAll(p)
			if p.FormName() == "file" {
				fileName, fileType, fileBody = p.FileName(), p.Header.Get("Content-Type"), string(b)
				continue
			}
			fields[p.FormName()] = string(b)
		}
		_, _ = w.Write([]byte(`{"id":"media.123"}`))
	}))
	defer srv.Close()

	g := NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret")
	data := "%PDF-1.7 fake"
	id, err := g.UploadMedia(t.Context(), "tok", "1001", "application/pdf", `in"voice.pdf`, strings.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if id != "media.123" {
		t.Fatalf("id %q", id)
	}
	if path != "/v24.0/1001/media" || auth != "Bearer tok" || proof != g.proof("tok") {
		t.Fatalf("path %q auth %q proof %q", path, auth, proof)
	}
	if fields["messaging_product"] != "whatsapp" || fields["type"] != "application/pdf" || len(fields) != 2 {
		t.Fatalf("fields %v", fields)
	}
	if fileName != `in"voice.pdf` || fileType != "application/pdf" || fileBody != data {
		t.Fatalf("file %q %q %q", fileName, fileType, fileBody)
	}
	if contentLength <= int64(len(data)) {
		t.Fatalf("content length %d not set", contentLength)
	}
}

func TestFetchMedia(t *testing.T) {
	var lookupAuth, downloadAuth, downloadProof string
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v24.0/media.9":
			lookupAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"messaging_product":"whatsapp","url":"` + srvURL + `/download/abc?ext=1","mime_type":"image/jpeg","sha256":"x","file_size":5,"id":"media.9"}`))
		case "/download/abc":
			downloadAuth, downloadProof = r.Header.Get("Authorization"), r.URL.Query().Get("appsecret_proof")
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("JPEG!"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	g := NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret")
	body, ct, size, err := g.FetchMedia(t.Context(), "tok", "media.9")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "JPEG!" || ct != "image/jpeg" || size != 5 {
		t.Fatalf("body %q type %q size %d", b, ct, size)
	}
	if lookupAuth != "Bearer tok" || downloadAuth != "Bearer tok" || downloadProof != "" {
		t.Fatalf("lookup auth %q download auth %q proof %q", lookupAuth, downloadAuth, downloadProof)
	}
}

func TestFetchMediaTooLarge(t *testing.T) {
	downloaded := false
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			downloaded = true
			return
		}
		_, _ = w.Write([]byte(`{"url":"` + srvURL + `/download","mime_type":"video/mp4","file_size":104857601}`))
	}))
	defer srv.Close()
	srvURL = srv.URL

	g := NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret")
	if _, _, _, err := g.FetchMedia(t.Context(), "tok", "media.9"); !errors.Is(err, model.ErrMediaTooLarge) {
		t.Fatalf("err = %v; want ErrMediaTooLarge", err)
	}
	if downloaded {
		t.Fatal("downloaded a file over the cap")
	}
}

func TestClassifySend(t *testing.T) {
	refuse := func(status int, body string) error {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		defer srv.Close()
		g := NewGraph(srv.Client(), srv.URL+"/v24.0", "app", "secret")
		_, err := g.SendText(t.Context(), "tok", "1001", "+14155550142", "hi", "")
		return classifySend(err)
	}

	err := refuse(400, `{"error":{"message":"(#131047) Re-engagement message","type":"OAuthException","code":131047,
		"error_data":{"messaging_product":"whatsapp","details":"Message failed to send because more than 24 hours have passed since the customer last replied to this number."},
		"fbtrace_id":"A"}}`)
	var se *model.SendError
	if !errors.As(err, &se) || !se.Permanent || se.Code != 131047 || se.Title != "Re-engagement message" ||
		!strings.HasPrefix(se.Message, "Message failed to send") {
		t.Fatalf("131047: %#v", err)
	}

	err = refuse(400, `{"error":{"message":"(#131016) Service unavailable","code":131016}}`)
	if !errors.As(err, &se) || se.Permanent || se.Code != 131016 {
		t.Fatalf("131016: %#v", err)
	}

	err = refuse(400, `{"error":{"message":"(#130429) Rate limit hit","code":130429}}`)
	if !errors.As(err, &se) || se.Permanent {
		t.Fatalf("130429: %#v", err)
	}

	err = refuse(401, `{"error":{"message":"Error validating access token","type":"OAuthException","code":190}}`)
	if !errors.As(err, &se) || !se.Permanent || se.Code != 190 {
		t.Fatalf("190: %#v", err)
	}

	err = refuse(503, `<html>bad gateway</html>`)
	if errors.As(err, &se) || err == nil {
		t.Fatalf("503: %#v; want a plain error", err)
	}

	if err := classifySend(errors.New("dial tcp: refused")); errors.As(err, &se) {
		t.Fatalf("transport: %#v; want a plain error", err)
	}
}
