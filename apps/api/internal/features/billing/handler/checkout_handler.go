package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// createCheckout: POST /checkout-sessions (svc.CreateCheckout).
func (h *Handler) createCheckout(ctx context.Context, in *dto.CreateCheckoutInput) (*dto.StartedCheckoutOutput, error) {
	userID, _ := authctx.UserID(ctx)
	c, err := h.svc.CreateCheckout(ctx, in.WorkspaceID, userID, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToStartedCheckoutOutput(c), nil
}

// completeTestCheckout: POST /checkout-sessions/{sessionId}/test-complete
// (svc.CompleteTestCheckout). Development's test gateway only.
func (h *Handler) completeTestCheckout(ctx context.Context, in *dto.CheckoutSessionInput) (*dto.CheckoutSessionOutput, error) {
	c, err := h.svc.CompleteTestCheckout(ctx, in.WorkspaceID, in.SessionID)
	if err != nil {
		return nil, err
	}
	return &dto.CheckoutSessionOutput{Body: dto.ToCheckoutSessionDTO(c)}, nil
}

// checkoutSession: GET /checkout-sessions/{sessionId} (svc.CheckoutSession).
func (h *Handler) checkoutSession(ctx context.Context, in *dto.CheckoutSessionInput) (*dto.CheckoutSessionOutput, error) {
	c, err := h.svc.CheckoutSession(ctx, in.WorkspaceID, in.SessionID)
	if err != nil {
		return nil, err
	}
	return &dto.CheckoutSessionOutput{Body: dto.ToCheckoutSessionDTO(c)}, nil
}
