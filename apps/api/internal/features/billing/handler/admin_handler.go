package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

// Staff admin tools under /v1/admin (svc.Admin…, CreatePlan, …). RequireStaff
// runs first, so every request here has a signed-in staff user.

func (h *Handler) adminCatalog(ctx context.Context, _ *struct{}) (*dto.AdminCatalogOutput, error) {
	c, err := h.svc.AdminCatalog(ctx)
	if err != nil {
		return nil, err
	}
	return dto.ToAdminCatalogOutput(c), nil
}

func (h *Handler) createPlan(ctx context.Context, in *dto.CreatePlanInput) (*dto.CreatedOutput, error) {
	id, err := h.svc.CreatePlan(ctx, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToCreatedOutput(id), nil
}

func (h *Handler) createPrice(ctx context.Context, in *dto.CreatePriceInput) (*dto.CreatedOutput, error) {
	id, err := h.svc.CreatePrice(ctx, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToCreatedOutput(id), nil
}

func (h *Handler) createInstallmentOffer(ctx context.Context, in *dto.CreateInstallmentOfferInput) (*dto.CreatedOutput, error) {
	id, err := h.svc.CreateInstallmentOffer(ctx, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToCreatedOutput(id), nil
}

func (h *Handler) createProviderPrice(ctx context.Context, in *dto.CreateProviderPriceInput) (*dto.CreatedOutput, error) {
	id, err := h.svc.CreateProviderPrice(ctx, in.ToModel())
	if err != nil {
		return nil, err
	}
	return dto.ToCreatedOutput(id), nil
}

// deactivate takes one kind of catalog row off sale (service.Catalog…).
func (h *Handler) deactivate(kind string) func(context.Context, *dto.DeactivateCatalogRowInput) (*struct{}, error) {
	return func(ctx context.Context, in *dto.DeactivateCatalogRowInput) (*struct{}, error) {
		return nil, h.svc.DeactivateCatalogRow(ctx, kind, in.ID)
	}
}

func (h *Handler) adminDiscounts(ctx context.Context, _ *struct{}) (*dto.AdminDiscountsOutput, error) {
	ds, err := h.svc.ListDiscounts(ctx, false)
	if err != nil {
		return nil, err
	}
	return dto.ToAdminDiscountsOutput(ds), nil
}

func (h *Handler) createDiscount(ctx context.Context, in *dto.CreateDiscountInput) (*dto.AdminDiscountOutput, error) {
	userID, _ := authctx.UserID(ctx)
	d, err := h.svc.CreateDiscount(ctx, in.ToModel(userID))
	if err != nil {
		return nil, err
	}
	return dto.ToAdminDiscountOutput(d), nil
}

func (h *Handler) deactivateDiscount(ctx context.Context, in *dto.DeactivateDiscountInput) (*struct{}, error) {
	return nil, h.svc.DeactivateDiscount(ctx, in.Code)
}

func (h *Handler) adminWorkspaces(ctx context.Context, in *dto.AdminWorkspacesInput) (*dto.AdminWorkspacesOutput, error) {
	ws, err := h.svc.AdminWorkspaces(ctx, in.Search)
	if err != nil {
		return nil, err
	}
	return dto.ToAdminWorkspacesOutput(ws), nil
}

func (h *Handler) grantPlan(ctx context.Context, in *dto.ManualGrantInput) (*struct{}, error) {
	userID, _ := authctx.UserID(ctx)
	return nil, h.svc.GrantPlanManually(ctx, in.ToModel(userID))
}

func (h *Handler) endGrant(ctx context.Context, in *dto.AdminWorkspaceInput) (*struct{}, error) {
	return nil, h.svc.EndGrantManually(ctx, in.Body.WorkspaceID)
}

func (h *Handler) restoreGrant(ctx context.Context, in *dto.AdminWorkspaceInput) (*struct{}, error) {
	return nil, h.svc.RestoreGrantManually(ctx, in.Body.WorkspaceID)
}

// catalogKinds are the catalog row kinds by route segment, in registration order.
var catalogKinds = []struct{ path, kind, name string }{
	{"plans", service.CatalogPlan, "plan"},
	{"prices", service.CatalogPrice, "price"},
	{"installment-offers", service.CatalogOffer, "installment-offer"},
	{"provider-prices", service.CatalogProviderPrice, "provider-price"},
}

func (h *Handler) updatePlan(ctx context.Context, in *dto.UpdatePlanInput) (*struct{}, error) {
	userID, _ := authctx.UserID(ctx)
	return nil, h.svc.UpdatePlan(ctx, in.ToModel(userID))
}

func (h *Handler) setGrantOverrides(ctx context.Context, in *dto.GrantOverridesInput) (*struct{}, error) {
	userID, _ := authctx.UserID(ctx)
	return nil, h.svc.SetGrantOverrides(ctx, in.ToModel(userID))
}
