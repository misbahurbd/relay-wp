package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

type ListInvoicesInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	Cursor      string    `query:"cursor" maxLength:"200" doc:"next_cursor from the previous page"`
}

type InvoiceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true" doc:"The workspace this request is for"`
	InvoiceID   uuid.UUID `path:"invoiceId"`
}

type InvoiceDTO struct {
	ID             uuid.UUID  `json:"id"`
	Number         *string    `json:"number" doc:"The provider's invoice number"`
	SubscriptionID *uuid.UUID `json:"subscription_id"`
	OrderID        *uuid.UUID `json:"order_id"`
	InstallmentID  *uuid.UUID `json:"installment_id"`
	Provider       string     `json:"provider" enum:"stripe,creem,paypal,sslcommerz"`
	Status         string     `json:"status" enum:"open,paid,failed,void,partially_refunded,refunded,disputed"`
	SubtotalMinor  int64      `json:"subtotal_minor"`
	DiscountMinor  int64      `json:"discount_minor"`
	TaxMinor       int64      `json:"tax_minor"`
	TotalMinor     int64      `json:"total_minor" doc:"subtotal - discount + tax"`
	Currency       string     `json:"currency"`
	PeriodStart    *time.Time `json:"period_start"`
	PeriodEnd      *time.Time `json:"period_end"`
	ReceiptURL     *string    `json:"receipt_url"`
	PaidAt         *time.Time `json:"paid_at"`
	CreatedAt      time.Time  `json:"created_at"`
	BillTo         *BillToDTO `json:"bill_to,omitempty" doc:"Billing details as they were when the invoice was issued; single invoice only"`
}

func ToInvoiceDTO(i model.Invoice) InvoiceDTO {
	out := InvoiceDTO{
		ID: i.ID, Number: i.Number, SubscriptionID: i.SubscriptionID, OrderID: i.OrderID, InstallmentID: i.InstallmentID,
		Provider: string(i.Provider), Status: i.Status, SubtotalMinor: i.SubtotalMinor, DiscountMinor: i.DiscountMinor,
		TaxMinor: i.TaxMinor, TotalMinor: i.TotalMinor, Currency: i.Currency, PeriodStart: i.PeriodStart,
		PeriodEnd: i.PeriodEnd, ReceiptURL: i.ReceiptURL, PaidAt: i.PaidAt, CreatedAt: i.CreatedAt,
	}
	if i.BillTo != nil {
		b := BillToDTO(*i.BillTo)
		out.BillTo = &b
	}
	return out
}

type InvoiceOutput struct {
	Body InvoiceDTO
}

type InvoicesOutput struct {
	Body struct {
		Invoices   []InvoiceDTO `json:"invoices"`
		NextCursor *string      `json:"next_cursor" doc:"Null on the last page"`
	}
}

func ToInvoicesOutput(p model.InvoicePage) *InvoicesOutput {
	out := &InvoicesOutput{}
	out.Body.Invoices = make([]InvoiceDTO, 0, len(p.Invoices))
	for _, i := range p.Invoices {
		out.Body.Invoices = append(out.Body.Invoices, ToInvoiceDTO(i))
	}
	out.Body.NextCursor = nonEmpty(p.NextCursor)
	return out
}
