package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Invoice statuses (invoices.status).
const (
	InvoiceOpen              = "open"
	InvoicePaid              = "paid"
	InvoiceFailed            = "failed"
	InvoiceVoid              = "void"
	InvoicePartiallyRefunded = "partially_refunded"
	InvoiceRefunded          = "refunded"
	InvoiceDisputed          = "disputed"
	// InvoiceDisputeLost is final: a dispute took the money back to the customer.
	InvoiceDisputeLost = "dispute_lost"
)

// InvoicePageSize is how many invoices a page lists.
const InvoicePageSize = 25

// Invoice is one charge: a subscription period, an order payment or an installment.
// TotalMinor = SubtotalMinor - DiscountMinor + TaxMinor.
type Invoice struct {
	ID uuid.UUID
	// Number is Relay's (RLY-2026-000001), on paid invoices only (Q19).
	Number         *string
	SubscriptionID *uuid.UUID
	OrderID        *uuid.UUID
	InstallmentID  *uuid.UUID
	Provider       payment.Provider
	Status         string
	SubtotalMinor  int64
	DiscountMinor  int64
	TaxMinor       int64
	TotalMinor     int64
	Currency       string
	PeriodStart    *time.Time
	PeriodEnd      *time.Time
	// PDFURL is a short-lived download link, once the invoice document is ready.
	PDFURL    *string
	PaidAt    *time.Time
	CreatedAt time.Time
	// BillTo is set on a single invoice, not in lists.
	BillTo *BillTo
}

// InvoicePage is one page of a workspace's invoices, newest first.
type InvoicePage struct {
	Invoices []Invoice
	// NextCursor is empty on the last page.
	NextCursor string
}
