package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// quoteOrder: POST /orders/quote (svc.QuoteOrder).
func (h *Handler) quoteOrder(ctx context.Context, in *dto.QuoteOrderInput) (*dto.OrderQuoteOutput, error) {
	q, err := h.svc.QuoteOrder(ctx, in.WorkspaceID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToOrderQuoteOutput(q), nil
}

// currentOrder: GET /orders/current (svc.CurrentOrder).
func (h *Handler) currentOrder(ctx context.Context, in *dto.WorkspaceInput) (*dto.OrderOutput, error) {
	o, err := h.svc.CurrentOrder(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &dto.OrderOutput{Body: dto.ToOrderDTO(o)}, nil
}

// order: GET /orders/{orderId} (svc.Order).
func (h *Handler) order(ctx context.Context, in *dto.OrderInput) (*dto.OrderOutput, error) {
	o, err := h.svc.Order(ctx, in.WorkspaceID, in.OrderID)
	if err != nil {
		return nil, err
	}
	return &dto.OrderOutput{Body: dto.ToOrderDTO(o)}, nil
}

// payInstallment: POST /orders/{orderId}/installments/{installmentId}/pay (svc.PayInstallment).
func (h *Handler) payInstallment(ctx context.Context, in *dto.PayInstallmentInput) (*dto.StartedCheckoutOutput, error) {
	userID, _ := authctx.UserID(ctx)
	c, err := h.svc.PayInstallment(ctx, in.WorkspaceID, userID, in.OrderID, in.InstallmentID, in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return dto.ToStartedCheckoutOutput(c), nil
}

// resumeOrder: POST /orders/{orderId}/resume (svc.ResumeOrder).
func (h *Handler) resumeOrder(ctx context.Context, in *dto.ResumeOrderInput) (*dto.StartedCheckoutOutput, error) {
	userID, _ := authctx.UserID(ctx)
	c, err := h.svc.ResumeOrder(ctx, in.WorkspaceID, userID, in.OrderID, in.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return dto.ToStartedCheckoutOutput(c), nil
}
