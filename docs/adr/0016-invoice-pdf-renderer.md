# 0016. Invoice PDF renderer

- **Status:** accepted
- **Date:** 2026-11-07

## Context

Relay issues its own invoices instead of forwarding the payment provider's receipt. Every paid invoice gets a Relay number (`RLY-2026-000001`), a PDF in object storage, and a "payment received" email with the PDF attached. The billing page downloads the same PDF.

A good-looking PDF needs a browser engine to lay out HTML and CSS. Running headless Chrome on the API box is out of the question: the API runs on 0.5 vCPU and 1 GB RAM (ARCHITECTURE.md, resource budget), and one Chrome tab can take hundreds of megabytes. Pure-Go PDF libraries avoid the browser but make the invoice a hand-drawn layout that is hard to change.

Cloudflare Browser Rendering has a REST endpoint that takes HTML and returns a PDF (`POST /client/v4/accounts/{account_id}/browser-rendering/pdf`). The browser runs on Cloudflare; Relay sends a few kilobytes of HTML and stores the bytes it gets back. Local development and tests must work without a Cloudflare account and without a browser.

`CLAUDE.md` section 2 allows a factory only when all three of its conditions hold. All three do here:

1. There are two real implementations: Cloudflare for production and a browser-free local stand-in for development and tests.
2. The implementation is selected by config (`PDF_RENDERER`).
3. It is built once at startup.

## Decision

- `platform/pdf/pdf.go` declares `Renderer { Render(ctx, html) ([]byte, error) }` and `New(cfg)`, a plain `switch` on `PDF_RENDERER`. One file per implementation: `cloudflare.go`, `local.go`.
- `cloudflare` posts `{html, pdfOptions: {format: "a4", printBackground: true, margin}}` with a Bearer token and returns the response body. It uses one `http.Client` with a 60-second timeout. A non-2xx response is an error that quotes the status and at most 512 bytes of the body, never the token. A 2xx body that doesn't start with `%PDF-` is an error too. `CLOUDFLARE_ACCOUNT_ID` and `CLOUDFLARE_API_TOKEN` are required for this driver; missing either fails at startup.
- `local` runs no browser. It writes a small valid PDF (A4, Helvetica) holding the HTML's text with the tags stripped, headed by a line saying the real layout renders through Cloudflare. It is the default, so a fresh checkout works with no keys.
- Rendering happens only in the `billing_invoice_document` River job, which is enqueued in the same transaction that records the paid invoice. Checkout and webhooks never wait on it. The job is idempotent: it renders only while `invoices.pdf_key` is NULL and emails only while `invoices.emailed_at` is NULL, and a failure returns an error so River retries.
- The PDF is stored at `invoices/<workspace_id>/<invoice_id>.pdf`. The API hands out a short-lived signed storage link (`Storage.SignedURL`), so downloads never pass through the API.
- `internal/app` builds the renderer and passes it to the billing service. Services hold the built instance; they never call `pdf.New`.

## Consequences

- The API box does no layout work: a render costs one outbound HTTPS call from a worker, and a download costs nothing on the API.
- The invoice is ordinary HTML and CSS (`features/billing/service/invoice_document.html`), so changing its look is a template edit.
- Locally, PDFs are plain-text stand-ins. To see the real layout, set `PDF_RENDERER=cloudflare` with a test account's keys.
- What to watch: Cloudflare's rate limits and outages delay PDFs, not payments; River retries with backoff for up to 12 attempts. The product docs also show the endpoint under `/browser-run/pdf`; if Cloudflare retires the `/browser-rendering/` path, only the URL in `cloudflare.go` changes.
