package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// listInvoices: GET /invoices (svc.Invoices).
func (h *Handler) listInvoices(ctx context.Context, in *dto.ListInvoicesInput) (*dto.InvoicesOutput, error) {
	p, err := h.svc.Invoices(ctx, in.WorkspaceID, in.Cursor)
	if err != nil {
		return nil, err
	}
	return dto.ToInvoicesOutput(p), nil
}

// invoice: GET /invoices/{invoiceId} (svc.Invoice).
func (h *Handler) invoice(ctx context.Context, in *dto.InvoiceInput) (*dto.InvoiceOutput, error) {
	i, err := h.svc.Invoice(ctx, in.WorkspaceID, in.InvoiceID)
	if err != nil {
		return nil, err
	}
	return &dto.InvoiceOutput{Body: dto.ToInvoiceDTO(i)}, nil
}
