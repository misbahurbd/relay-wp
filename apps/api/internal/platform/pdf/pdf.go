// Package pdf turns an HTML document into a PDF. The implementation is chosen
// by PDF_RENDERER (docs/adr/0016-invoice-pdf-renderer.md). No implementation
// runs a browser on Relay's servers.
package pdf

import (
	"context"
	"fmt"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
)

// Renderer renders a self-contained HTML document (inline CSS, no external
// assets) as an A4 PDF.
type Renderer interface {
	Render(ctx context.Context, html string) ([]byte, error)
}

// New returns the Renderer for cfg.Renderer: "cloudflare" or "local".
func New(cfg config.PDF) (Renderer, error) {
	switch cfg.Renderer {
	case "cloudflare":
		return newCloudflare(cfg, cloudflareAPI)
	case "local":
		return localRenderer{}, nil
	default:
		return nil, fmt.Errorf("unknown PDF_RENDERER %q (want cloudflare or local)", cfg.Renderer)
	}
}
