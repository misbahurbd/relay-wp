package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Invoices: built in Wave 0 (no track owns these reads). Tracks that issue
// invoices write them through the repository in their own transactions.

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
		page.Invoices = append(page.Invoices, model.Invoice{
			ID: r.ID, Number: r.Number, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID,
			InstallmentID: r.InstallmentID, Provider: payment.Provider(r.Provider), Status: r.Status,
			SubtotalMinor: r.SubtotalMinor, DiscountMinor: r.DiscountMinor, TaxMinor: r.TaxMinor,
			TotalMinor: r.TotalMinor, Currency: r.Currency, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
			ReceiptURL: r.ReceiptUrl, PaidAt: r.PaidAt, CreatedAt: r.CreatedAt,
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
	return model.Invoice{
		ID: r.ID, Number: r.Number, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID,
		InstallmentID: r.InstallmentID, Provider: payment.Provider(r.Provider), Status: r.Status,
		SubtotalMinor: r.SubtotalMinor, DiscountMinor: r.DiscountMinor, TaxMinor: r.TaxMinor,
		TotalMinor: r.TotalMinor, Currency: r.Currency, PeriodStart: r.PeriodStart, PeriodEnd: r.PeriodEnd,
		ReceiptURL: r.ReceiptUrl, PaidAt: r.PaidAt, CreatedAt: r.CreatedAt, BillTo: &billTo,
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
