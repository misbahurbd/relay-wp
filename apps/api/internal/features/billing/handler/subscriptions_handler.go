package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

// Track T6 builds these. Until then each answers 501 not_implemented.

// overview: GET /v1/workspace/billing (svc.Overview).
func (h *Handler) overview(context.Context, *dto.WorkspaceInput) (*dto.OverviewOutput, error) {
	return nil, model.ErrNotImplemented
}

// subscription: GET /subscription (svc.Subscription).
func (h *Handler) subscription(context.Context, *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	return nil, model.ErrNotImplemented
}

// previewChange: POST /subscription/change-preview (svc.PreviewChange).
func (h *Handler) previewChange(context.Context, *dto.ChangePlanInput) (*dto.ChangeQuoteOutput, error) {
	return nil, model.ErrNotImplemented
}

// changePlan: POST /subscription/change (svc.ChangePlan).
func (h *Handler) changePlan(context.Context, *dto.ChangePlanInput) (*dto.SubscriptionOutput, error) {
	return nil, model.ErrNotImplemented
}

// cancelPendingChange: DELETE /subscription/pending-change (svc.CancelPendingChange).
func (h *Handler) cancelPendingChange(context.Context, *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	return nil, model.ErrNotImplemented
}

// cancelSubscription: POST /subscription/cancel (svc.CancelSubscription).
func (h *Handler) cancelSubscription(context.Context, *dto.CancelSubscriptionInput) (*dto.SubscriptionOutput, error) {
	return nil, model.ErrNotImplemented
}

// resumeSubscription: POST /subscription/resume (svc.ResumeSubscription).
func (h *Handler) resumeSubscription(context.Context, *dto.WorkspaceInput) (*dto.SubscriptionOutput, error) {
	return nil, model.ErrNotImplemented
}
