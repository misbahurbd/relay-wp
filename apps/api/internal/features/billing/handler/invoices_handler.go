package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// listInvoices: GET /invoices (svc.Invoices).
func (h *Handler) listInvoices(context.Context, *dto.ListInvoicesInput) (*dto.InvoicesOutput, error) {
	return nil, model.ErrNotImplemented
}

// invoice: GET /invoices/{invoiceId} (svc.Invoice).
func (h *Handler) invoice(context.Context, *dto.InvoiceInput) (*dto.InvoiceOutput, error) {
	return nil, model.ErrNotImplemented
}
