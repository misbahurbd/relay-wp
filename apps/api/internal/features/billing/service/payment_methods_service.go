package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

// Saved cards (docs/PHASE-7-BILLING.md section 4). The card stays at the
// provider: Relay reads it on each request and never stores or logs it.

// PaymentMethod returns the card the workspace's provider will charge next.
// A provider that fails or shares no card details gives a nil Card, so the
// billing page still loads.
func (s *Service) PaymentMethod(ctx context.Context, workspaceID uuid.UUID) (model.PaymentMethod, error) {
	c, g, err := s.cardCustomer(ctx, workspaceID)
	if errors.Is(err, model.ErrNoCardCustomer) {
		return model.PaymentMethod{CardDetails: true}, nil
	}
	if err != nil {
		return model.PaymentMethod{}, err
	}
	out := model.PaymentMethod{Provider: c.Provider, CardDetails: true, CanManage: true}
	req := payment.CardRequest{ExternalCustomerID: *c.ExternalCustomerID}
	if c.ExternalSubscriptionID != nil {
		req.ExternalSubscriptionID = *c.ExternalSubscriptionID
	}
	pm, err := g.Card(ctx, req)
	switch {
	case errors.Is(err, payment.ErrUnsupported):
		out.CardDetails = false
	case err != nil:
		s.logger().Warn("read card from payment provider", "workspace_id", workspaceID, "provider", c.Provider, "err", err)
	case pm != nil:
		out.Card = &model.Card{Brand: pm.Brand, Last4: pm.Last4, ExpMonth: pm.ExpMonth, ExpYear: pm.ExpYear}
	}
	return out, nil
}

// PortalURL returns a link to the provider's page where the customer manages
// their cards. model.ErrNoCardCustomer before anything was bought.
func (s *Service) PortalURL(ctx context.Context, workspaceID uuid.UUID) (string, error) {
	c, g, err := s.cardCustomer(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	url, err := g.PortalURL(ctx, payment.PortalRequest{
		ExternalCustomerID: *c.ExternalCustomerID,
		ReturnURL:          strings.TrimRight(s.cfg.AppBaseURL, "/") + "/billing?from=portal",
	})
	if err != nil {
		return "", s.gatewayErr("open customer portal", err)
	}
	return url, nil
}

// cardCustomer finds the provider customer whose card pays next and its
// gateway. model.ErrNoCardCustomer when there is none, or its provider is no
// longer enabled.
func (s *Service) cardCustomer(ctx context.Context, workspaceID uuid.UUID) (gen.GetCardCustomerRow, payment.Gateway, error) {
	c, err := repository.NewRepository(s.pool).GetCardCustomer(ctx, workspaceID)
	if repository.IsNoRows(err) || (err == nil && c.ExternalCustomerID == nil) {
		return c, nil, model.ErrNoCardCustomer
	}
	if err != nil {
		return c, nil, fmt.Errorf("get card customer: %w", err)
	}
	p, ok := payment.ParseProvider(c.Provider)
	if !ok {
		return c, nil, model.ErrNoCardCustomer
	}
	g, ok := s.payments.Get(p)
	if !ok {
		return c, nil, model.ErrNoCardCustomer
	}
	return c, g, nil
}
