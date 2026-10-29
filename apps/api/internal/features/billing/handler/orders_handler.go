package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// quoteOrder: POST /orders/quote (svc.QuoteOrder).
func (h *Handler) quoteOrder(context.Context, *dto.QuoteOrderInput) (*dto.OrderQuoteOutput, error) {
	return nil, model.ErrNotImplemented
}

// currentOrder: GET /orders/current (svc.CurrentOrder).
func (h *Handler) currentOrder(context.Context, *dto.WorkspaceInput) (*dto.OrderOutput, error) {
	return nil, model.ErrNotImplemented
}

// order: GET /orders/{orderId} (svc.Order).
func (h *Handler) order(context.Context, *dto.OrderInput) (*dto.OrderOutput, error) {
	return nil, model.ErrNotImplemented
}

// payInstallment: POST /orders/{orderId}/installments/{installmentId}/pay (svc.PayInstallment).
func (h *Handler) payInstallment(context.Context, *dto.PayInstallmentInput) (*dto.StartedCheckoutOutput, error) {
	return nil, model.ErrNotImplemented
}

// resumeOrder: POST /orders/{orderId}/resume (svc.ResumeOrder).
func (h *Handler) resumeOrder(context.Context, *dto.ResumeOrderInput) (*dto.StartedCheckoutOutput, error) {
	return nil, model.ErrNotImplemented
}
