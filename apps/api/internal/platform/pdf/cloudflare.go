package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

const (
	cloudflareAPI = "https://api.cloudflare.com/client/v4"
	// maxPDFSize caps what is read back; an invoice is tens of kilobytes.
	maxPDFSize = 20 << 20
	// errBodyLimit is how much of a failed response an error quotes.
	errBodyLimit = 512
)

// cloudflare renders through Cloudflare Browser Rendering's PDF endpoint:
// the browser runs on Cloudflare, so Relay only sends HTML and gets bytes back.
type cloudflare struct {
	client   *http.Client
	endpoint string
	token    string
}

func newCloudflare(cfg config.PDF, base string) (*cloudflare, error) {
	if cfg.CloudflareAccountID == "" || cfg.CloudflareAPIToken == "" {
		return nil, errors.New("pdf: the cloudflare renderer needs CLOUDFLARE_ACCOUNT_ID and CLOUDFLARE_API_TOKEN")
	}
	return &cloudflare{
		client:   &http.Client{Timeout: 60 * time.Second},
		endpoint: base + "/accounts/" + url.PathEscape(cfg.CloudflareAccountID) + "/browser-rendering/pdf",
		token:    cfg.CloudflareAPIToken,
	}, nil
}

type cloudflareRequest struct {
	HTML       string            `json:"html"`
	PDFOptions cloudflarePDFOpts `json:"pdfOptions"`
}

type cloudflarePDFOpts struct {
	Format          string            `json:"format"`
	PrintBackground bool              `json:"printBackground"`
	Margin          map[string]string `json:"margin"`
}

func (c *cloudflare) Render(ctx context.Context, html string) ([]byte, error) {
	body, err := json.Marshal(cloudflareRequest{HTML: html, PDFOptions: cloudflarePDFOpts{
		Format: "a4", PrintBackground: true,
		Margin: map[string]string{"top": "16mm", "right": "16mm", "bottom": "16mm", "left": "16mm"},
	}})
	if err != nil {
		return nil, fmt.Errorf("pdf: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("pdf: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pdf: cloudflare: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, errBodyLimit))
		return nil, fmt.Errorf("pdf: cloudflare returned %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	out, err := io.ReadAll(io.LimitReader(resp.Body, maxPDFSize+1))
	if err != nil {
		return nil, fmt.Errorf("pdf: read cloudflare response: %w", err)
	}
	if len(out) > maxPDFSize {
		return nil, fmt.Errorf("pdf: cloudflare response is over %d bytes", maxPDFSize)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		return nil, fmt.Errorf("pdf: cloudflare returned %d bytes that are not a PDF", len(out))
	}
	return out, nil
}
