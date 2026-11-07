package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Invoices: built in Wave 0 (no track owns these reads). Every invoice is
// written through issueInvoice, in the transaction that records its payment.

// pdfLinkTTL is how long an invoice's download link works.
const pdfLinkTTL = 15 * time.Minute

// issueInvoice records one invoice in tx. A paid invoice also takes the next
// number of its year (UTC, of paid_at) and queues its document job, both in
// tx: a rollback gives the number back, and a replay, which inserts no row,
// takes none. False means the provider's payment is already recorded.
func (s *Service) issueInvoice(ctx context.Context, tx pgx.Tx, p gen.CreateInvoiceParams) (bool, error) {
	r := repository.NewRepository(tx)
	id, err := r.CreateInvoice(ctx, p)
	if repository.IsNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create invoice: %w", err)
	}
	if p.Status != model.InvoicePaid {
		return true, nil
	}
	if p.PaidAt == nil {
		return false, fmt.Errorf("paid invoice %s has no paid_at", id)
	}
	year := p.PaidAt.UTC().Year()
	seq, err := r.NextInvoiceSeq(ctx, int32(year)) //nolint:gosec // a calendar year fits in int32
	if err != nil {
		return false, fmt.Errorf("next invoice number: %w", err)
	}
	if _, err := r.SetInvoiceNumber(ctx, gen.SetInvoiceNumberParams{
		Number: invoiceNumber(s.cfg.InvoicePrefix, year, seq), WorkspaceID: p.WorkspaceID, InvoiceID: id,
	}); err != nil {
		return false, fmt.Errorf("set invoice number: %w", err)
	}
	if _, err := s.queue.InsertTx(ctx, tx, model.InvoiceDocumentArgs{WorkspaceID: p.WorkspaceID, InvoiceID: id}, nil); err != nil {
		return false, fmt.Errorf("enqueue invoice document: %w", err)
	}
	return true, nil
}

// invoiceNumber is "RLY-2026-000001"; the sequence grows past six digits if it must.
func invoiceNumber(prefix string, year int, seq int32) *string {
	n := fmt.Sprintf("%s-%d-%06d", prefix, year, seq)
	return &n
}

// pdfURL is a download link for the invoice's stored PDF, or nil before the
// document job has stored one.
func (s *Service) pdfURL(ctx context.Context, key, number *string) (*string, error) {
	if key == nil {
		return nil, nil
	}
	name := "invoice.pdf"
	if number != nil {
		name = *number + ".pdf"
	}
	u, err := s.docs.Storage.SignedURL(ctx, *key, name, pdfLinkTTL)
	if err != nil {
		return nil, fmt.Errorf("invoice pdf link: %w", err)
	}
	return &u, nil
}

// Invoices lists the workspace's invoices, newest first, one page at a time.
// cursor is empty for the first page, then the previous page's NextCursor.
func (s *Service) Invoices(ctx context.Context, workspaceID uuid.UUID, cursor string) (model.InvoicePage, error) {
	params := gen.ListInvoicesParams{WorkspaceID: workspaceID, MaxRows: model.InvoicePageSize + 1}
	if cursor != "" {
		at, id, err := decodeInvoiceCursor(cursor)
		if err != nil {
			return model.InvoicePage{}, model.ErrInvalidCursor
		}
		params.CursorCreatedAt, params.CursorID = &at, &id
	}
	rows, err := repository.NewRepository(s.pool).ListInvoices(ctx, params)
	if err != nil {
		return model.InvoicePage{}, fmt.Errorf("list invoices: %w", err)
	}
	page := model.InvoicePage{Invoices: make([]model.Invoice, 0, min(len(rows), model.InvoicePageSize))}
	for i, r := range rows {
		if i == model.InvoicePageSize {
			last := page.Invoices[len(page.Invoices)-1]
			page.NextCursor = encodeInvoiceCursor(last.CreatedAt, last.ID)
			break
		}
		link, err := s.pdfURL(ctx, r.PdfKey, r.Number)
		if err != nil {
			return model.InvoicePage{}, err
		}
		page.Invoices = append(page.Invoices, model.Invoice{
			ID: r.ID, Number: r.Number, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID,
			InstallmentID: r.InstallmentID, Provider: payment.Provider(r.Provider), Status: r.Status,
			SubtotalMinor: r.SubtotalMinor, DiscountMinor: r.DiscountMinor, TaxMinor: r.TaxMinor,
			TotalMinor: r.TotalMinor, Currency: r.Currency, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
			PDFURL: link, PaidAt: r.PaidAt, CreatedAt: r.CreatedAt,
		})
	}
	return page, nil
}

// Invoice returns one of the workspace's invoices with its bill_to.
func (s *Service) Invoice(ctx context.Context, workspaceID, invoiceID uuid.UUID) (model.Invoice, error) {
	r, err := repository.NewRepository(s.pool).GetInvoice(ctx, gen.GetInvoiceParams{
		WorkspaceID: workspaceID, InvoiceID: invoiceID,
	})
	if repository.IsNoRows(err) {
		return model.Invoice{}, model.ErrInvoiceNotFound
	}
	if err != nil {
		return model.Invoice{}, fmt.Errorf("get invoice: %w", err)
	}
	var billTo model.BillTo
	if err := json.Unmarshal(r.BillTo, &billTo); err != nil {
		return model.Invoice{}, fmt.Errorf("invoice %s bill_to: %w", r.ID, err)
	}
	link, err := s.pdfURL(ctx, r.PdfKey, r.Number)
	if err != nil {
		return model.Invoice{}, err
	}
	return model.Invoice{
		ID: r.ID, Number: r.Number, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID,
		InstallmentID: r.InstallmentID, Provider: payment.Provider(r.Provider), Status: r.Status,
		SubtotalMinor: r.SubtotalMinor, DiscountMinor: r.DiscountMinor, TaxMinor: r.TaxMinor,
		TotalMinor: r.TotalMinor, Currency: r.Currency, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
		PDFURL: link, PaidAt: r.PaidAt, CreatedAt: r.CreatedAt, BillTo: &billTo,
	}, nil
}

// The cursor is the last row's created_at and id, opaque to clients.
func encodeInvoiceCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

func decodeInvoiceCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor: %w", err)
	}
	ts, rawID, ok := strings.Cut(string(raw), "|")
	if !ok {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor: no separator")
	}
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor time: %w", err)
	}
	id, err := uuid.Parse(rawID)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("cursor id: %w", err)
	}
	return at, id, nil
}
