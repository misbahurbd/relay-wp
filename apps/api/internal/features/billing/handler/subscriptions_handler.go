package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// overview: GET /v1/workspace/billing (svc.Overview).
func (h *Handler) overview(ctx context.Context, in *dto.WorkspaceInput) (*dto.OverviewOutput, error) {
	o, err := h.svc.Overview(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return dto.ToOverviewOutput(o), nil
}

// subscription: GET /subscription (svc.Subscription).
func (h *Handler) subscription(ctx context.Context, in *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	return subscriptionOutput(h.svc.Subscription(ctx, in.WorkspaceID))
}

// previewChange: POST /subscription/change-preview (svc.PreviewChange).
func (h *Handler) previewChange(ctx context.Context, in *dto.ChangePlanInput) (*dto.ChangeQuoteOutput, error) {
	q, err := h.svc.PreviewChange(ctx, in.WorkspaceID, in.Body.PlanPriceID)
	if err != nil {
		return nil, err
	}
	return dto.ToChangeQuoteOutput(q), nil
}

// changePlan: POST /subscription/change (svc.ChangePlan).
func (h *Handler) changePlan(ctx context.Context, in *dto.ChangePlanInput) (*dto.SubscriptionOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return subscriptionOutput(h.svc.ChangePlan(ctx, in.WorkspaceID, userID, in.Body.PlanPriceID))
}

// cancelPendingChange: DELETE /subscription/pending-change (svc.CancelPendingChange).
func (h *Handler) cancelPendingChange(ctx context.Context, in *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return subscriptionOutput(h.svc.CancelPendingChange(ctx, in.WorkspaceID, userID))
}

// cancelSubscription: POST /subscription/cancel (svc.CancelSubscription).
func (h *Handler) cancelSubscription(ctx context.Context, in *dto.CancelSubscriptionInput) (*dto.SubscriptionOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return subscriptionOutput(h.svc.CancelSubscription(ctx, in.WorkspaceID, userID, in.Body.AtPeriodEnd))
}

// resumeSubscription: POST /subscription/resume (svc.ResumeSubscription).
func (h *Handler) resumeSubscription(ctx context.Context, in *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	userID, _ := authctx.UserID(ctx)
	return subscriptionOutput(h.svc.ResumeSubscription(ctx, in.WorkspaceID, userID))
}

func subscriptionOutput(s model.Subscription, err error) (*dto.SubscriptionOutput, error) {
	if err != nil {
		return nil, err
	}
	return &dto.SubscriptionOutput{Body: dto.ToSubscriptionDTO(s)}, nil
}
