package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

//go:embed invoice_document.html
var invoiceHTML string

var invoiceTemplate = template.Must(template.New("invoice").Parse(invoiceHTML))

// invoiceDoc is what the invoice PDF and its email show, formatted.
type invoiceDoc struct {
	Number    string
	IssueDate string
	Seller    Seller
	// BillTo is the bill_to snapshot as printed lines, legal name first.
	BillTo      []string
	Email       string
	Description string
	Subtotal    string
	// Discount and Tax are empty when zero, so their lines are left out.
	Discount string
	Tax      string
	Total    string
	Provider string
}

// DeliverInvoiceDocument renders a paid invoice's PDF, stores it and emails
// it to the billing contact. Each step runs once: a stored pdf_key is never
// rendered again, and a set emailed_at is never sent again, so River can
// retry the whole job after any failure.
func (s *Service) DeliverInvoiceDocument(ctx context.Context, workspaceID, invoiceID uuid.UUID) error {
	r := repository.NewRepository(s.pool)
	row, err := r.GetInvoiceDocument(ctx, gen.GetInvoiceDocumentParams{WorkspaceID: workspaceID, InvoiceID: invoiceID})
	if repository.IsNoRows(err) {
		return model.ErrInvoiceNotFound
	}
	if err != nil {
		return fmt.Errorf("load invoice document: %w", err)
	}
	doc, err := s.invoiceDoc(row)
	if err != nil {
		return err
	}

	var file []byte
	if row.PdfKey == nil {
		if file, err = s.renderInvoice(ctx, doc); err != nil {
			return err
		}
		key := fmt.Sprintf("invoices/%s/%s.pdf", workspaceID, invoiceID)
		if err := s.docs.Storage.Put(ctx, key, bytes.NewReader(file), int64(len(file)), "application/pdf"); err != nil {
			return fmt.Errorf("store invoice pdf: %w", err)
		}
		// A run that lost a race stored the same bytes under the same key.
		if _, err := r.SetInvoicePDFKey(ctx, gen.SetInvoicePDFKeyParams{
			PdfKey: &key, WorkspaceID: workspaceID, InvoiceID: invoiceID,
		}); err != nil {
			return fmt.Errorf("save invoice pdf key: %w", err)
		}
	}

	if row.EmailedAt != nil || doc.Email == "" {
		return nil
	}
	if file == nil {
		if file, err = s.storedPDF(ctx, *row.PdfKey); err != nil {
			return err
		}
	}
	// A crash between Send and the update below sends the email once more on
	// retry. That beats losing it, and no lock is held across the SMTP call.
	if err := s.docs.Mailer.Send(ctx, invoiceReceiptEmail(doc, file, s.billingURL(workspaceID))); err != nil {
		return fmt.Errorf("send invoice email: %w", err)
	}
	if _, err := r.MarkInvoiceEmailed(ctx, gen.MarkInvoiceEmailedParams{
		EmailedAt: new(s.now()), WorkspaceID: workspaceID, InvoiceID: invoiceID,
	}); err != nil {
		return fmt.Errorf("mark invoice emailed: %w", err)
	}
	return nil
}

// invoiceDocumentGrace is how long an invoice's own document job has before
// the sweep queues another: longer than its 12 attempts take.
const invoiceDocumentGrace = 24 * time.Hour

// SweepInvoiceDocuments queues the document job for numbered invoices that
// still have no PDF a day after they were issued, and returns how many.
// Run hourly by the InvoiceDocumentsSweep worker.
func (s *Service) SweepInvoiceDocuments(ctx context.Context) (int, error) {
	rows, err := repository.NewRepository(s.pool).ListInvoicesWithoutPDF(ctx, gen.ListInvoicesWithoutPDFParams{
		Before: s.now().Add(-invoiceDocumentGrace), MaxRows: jobBatch,
	})
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		for _, row := range rows {
			args := model.InvoiceDocumentArgs{WorkspaceID: row.WorkspaceID, InvoiceID: row.ID}
			if _, err := s.queue.InsertTx(ctx, tx, args, nil); err != nil {
				return fmt.Errorf("enqueue invoice document: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

func (s *Service) renderInvoice(ctx context.Context, doc invoiceDoc) ([]byte, error) {
	var page strings.Builder
	if err := invoiceTemplate.Execute(&page, doc); err != nil {
		return nil, fmt.Errorf("invoice html: %w", err)
	}
	file, err := s.docs.Renderer.Render(ctx, page.String())
	if err != nil {
		return nil, fmt.Errorf("render invoice pdf: %w", err)
	}
	return file, nil
}

func (s *Service) storedPDF(ctx context.Context, key string) ([]byte, error) {
	rc, err := s.docs.Storage.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("read invoice pdf: %w", err)
	}
	defer func() { _ = rc.Close() }()
	file, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read invoice pdf: %w", err)
	}
	return file, nil
}

func (s *Service) invoiceDoc(row gen.GetInvoiceDocumentRow) (invoiceDoc, error) {
	if row.Number == nil || row.PaidAt == nil {
		return invoiceDoc{}, errors.New("invoice document: only a paid, numbered invoice has one")
	}
	var billTo model.BillTo
	if err := json.Unmarshal(row.BillTo, &billTo); err != nil {
		return invoiceDoc{}, fmt.Errorf("invoice %s bill_to: %w", row.ID, err)
	}
	doc := invoiceDoc{
		Number: *row.Number, IssueDate: row.PaidAt.UTC().Format("January 2, 2006"), Seller: s.cfg.Seller,
		BillTo: billToLines(billTo), Email: billTo.Email, Description: invoiceDescription(row),
		Subtotal: money(row.SubtotalMinor, row.Currency), Total: money(row.TotalMinor, row.Currency),
		Provider: providerName(payment.Provider(row.Provider)),
	}
	if row.DiscountMinor > 0 {
		doc.Discount = money(row.DiscountMinor, row.Currency)
	}
	if row.TaxMinor > 0 {
		doc.Tax = money(row.TaxMinor, row.Currency)
	}
	return doc, nil
}

// invoiceDescription is the invoice's one line item: "Growth plan · monthly ·
// Nov 7 – Dec 7, 2026", "Growth plan · lifetime deal · installment 2 of 3".
func invoiceDescription(row gen.GetInvoiceDocumentRow) string {
	parts := []string{planLabel(row.PlanName)}
	switch {
	case row.OrderID != nil:
		parts = append(parts, "lifetime deal")
		if row.InstallmentSeq != nil && row.InstallmentCount > 1 {
			parts = append(parts, fmt.Sprintf("installment %d of %d", *row.InstallmentSeq, row.InstallmentCount))
		}
	case row.PlanInterval != nil:
		years, months := 0, 1
		cadence := "monthly"
		if *row.PlanInterval == "year" {
			years, months, cadence = 1, 0, "yearly"
		}
		if row.PeriodStart == nil || row.PeriodEnd == nil {
			parts = append(parts, cadence)
			break
		}
		start, end := row.PeriodStart.UTC(), row.PeriodEnd.UTC()
		// Providers clamp month ends (Jan 31 → Feb 28), so allow a few days.
		if drift := end.Sub(start.AddDate(years, months, 0)); drift < -72*time.Hour || drift > 72*time.Hour {
			parts = append(parts, "change of plan (prorated)")
			break
		}
		parts = append(parts, cadence, dateRange(start, end))
	}
	return strings.Join(parts, " · ")
}

// planLabel is "Growth plan", for a plan named "Growth" or "Growth plan".
func planLabel(name string) string {
	if strings.HasSuffix(strings.ToLower(name), " plan") {
		return name
	}
	return name + " plan"
}

// dateRange is "Nov 7 – Dec 7, 2026", or "Dec 7, 2026 – Jan 7, 2027" across a year.
func dateRange(start, end time.Time) string {
	if start.Year() == end.Year() {
		return start.Format("Jan 2") + " – " + end.Format("Jan 2, 2006")
	}
	return start.Format("Jan 2, 2006") + " – " + end.Format("Jan 2, 2006")
}

// billToLines prints the bill_to snapshot: name, address, country, tax ID, email.
func billToLines(b model.BillTo) []string {
	str := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.TrimSpace(*p)
	}
	join := func(sep string, parts ...string) string {
		var kept []string
		for _, p := range parts {
			if p != "" {
				kept = append(kept, p)
			}
		}
		return strings.Join(kept, sep)
	}
	taxID := str(b.TaxID)
	if taxID != "" {
		label := "Tax ID"
		if t := str(b.TaxIDType); t != "" {
			label = strings.ToUpper(strings.ReplaceAll(t, "_", " "))
		}
		taxID = label + ": " + taxID
	}
	var lines []string
	for _, l := range []string{
		strings.TrimSpace(b.LegalName), str(b.AddressLine1), str(b.AddressLine2),
		join(" ", join(", ", str(b.City), str(b.State)), str(b.PostalCode)),
		str(b.Country), taxID, strings.TrimSpace(b.Email),
	} {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

// providerName is how the invoice names the payment provider.
func providerName(p payment.Provider) string {
	switch p {
	case payment.Stripe:
		return "Stripe"
	case payment.Creem:
		return "Creem"
	case payment.PayPal:
		return "PayPal"
	case payment.SSLCommerz:
		return "SSLCommerz"
	default:
		return string(p)
	}
}
