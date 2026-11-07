package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

func TestNewChoosesRenderer(t *testing.T) {
	if _, err := New(config.PDF{Renderer: "wkhtml"}); err == nil {
		t.Error("expected error for unknown renderer")
	}
	if _, err := New(config.PDF{Renderer: "cloudflare", CloudflareAccountID: "acc"}); err == nil {
		t.Error("expected error for cloudflare without an API token")
	}
	if _, err := New(config.PDF{Renderer: "cloudflare", CloudflareAPIToken: "tok"}); err == nil {
		t.Error("expected error for cloudflare without an account ID")
	}
	if _, err := New(config.PDF{Renderer: "local"}); err != nil {
		t.Errorf("local: %v", err)
	}
}

func TestLocalWritesAPDFWithTheText(t *testing.T) {
	r, err := New(config.PDF{Renderer: "local"})
	if err != nil {
		t.Fatal(err)
	}
	doc := `<html><head><style>body{color:red}</style></head><body><h1>Invoice</h1>` +
		`<p>RLY-2026-000001</p><table><tr><td>Growth plan &middot; monthly</td><td>$79.00</td></tr></table></body></html>`
	out, err := r.Render(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) || !bytes.HasSuffix(bytes.TrimSpace(out), []byte("%%EOF")) {
		t.Fatalf("not a PDF: %q…%q", out[:min(20, len(out))], out[max(0, len(out)-20):])
	}
	s := string(out)
	for _, want := range []string{"(Invoice) Tj", "(RLY-2026-000001) Tj", `Growth plan \267 monthly $79.00`, "Cloudflare Browser Rendering"} {
		if !strings.Contains(s, want) {
			t.Errorf("PDF lacks %q", want)
		}
	}
	if strings.Contains(s, "color:red") {
		t.Error("PDF contains the stylesheet")
	}
}

func TestLocalSplitsLongDocumentsIntoPages(t *testing.T) {
	out, err := localRenderer{}.Render(context.Background(), strings.Repeat("<p>line</p>", 200))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("/Count 4")) {
		t.Error("202 lines should fill 4 pages")
	}
}

func TestCloudflareSendsHTMLAndReturnsThePDF(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody cloudflareRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7 fake\n%%EOF"))
	}))
	defer srv.Close()

	c, err := newCloudflare(config.PDF{Renderer: "cloudflare", CloudflareAccountID: "acc_1", CloudflareAPIToken: "tok_secret"}, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	out, err := c.Render(context.Background(), "<p>Invoice</p>")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "%PDF-1.7 fake\n%%EOF" {
		t.Errorf("Render = %q", out)
	}
	if gotPath != "/accounts/acc_1/browser-rendering/pdf" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuth != "Bearer tok_secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotBody.HTML != "<p>Invoice</p>" || gotBody.PDFOptions.Format != "a4" || !gotBody.PDFOptions.PrintBackground {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestCloudflareErrorNamesStatusNotToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"code":2001,"message":"Rate limit exceeded"}]}` + strings.Repeat(" pad", 500)))
	}))
	defer srv.Close()

	c, err := newCloudflare(config.PDF{CloudflareAccountID: "acc_1", CloudflareAPIToken: "tok_secret"}, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Render(context.Background(), "<p>Invoice</p>")
	if err == nil {
		t.Fatal("expected an error for a 429")
	}
	msg := err.Error()
	if !strings.Contains(msg, "429") || !strings.Contains(msg, "Rate limit exceeded") {
		t.Errorf("error = %q", msg)
	}
	if strings.Contains(msg, "tok_secret") {
		t.Error("error contains the API token")
	}
	if len(msg) > errBodyLimit+100 {
		t.Errorf("error quotes %d bytes of the body", len(msg))
	}
}

func TestCloudflareRejectsANonPDFSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	c, err := newCloudflare(config.PDF{CloudflareAccountID: "acc_1", CloudflareAPIToken: "tok"}, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Render(context.Background(), "<p>x</p>"); err == nil {
		t.Error("expected an error for a body that is not a PDF")
	}
}
