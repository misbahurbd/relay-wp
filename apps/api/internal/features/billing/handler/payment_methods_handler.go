package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
)

// paymentMethod: GET /payment-method (svc.PaymentMethod).
func (h *Handler) paymentMethod(ctx context.Context, in *dto.WorkspaceInput) (*dto.PaymentMethodOutput, error) {
	m, err := h.svc.PaymentMethod(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToPaymentMethodOutput(m), nil
}

// portalSession: POST /portal-session (svc.PortalURL).
func (h *Handler) portalSession(ctx context.Context, in *dto.WorkspaceInput) (*dto.PortalSessionOutput, error) {
	url, err := h.svc.PortalURL(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	out := &dto.PortalSessionOutput{}
	out.Body.URL = url
	return out, nil
}
