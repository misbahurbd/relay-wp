package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/repository"
)

// triggers maps a provider and order status to the setting that wants it.
var triggers = map[string]string{
	"shopify/created":     model.TriggerShopifyOrderCreated,
	"shopify/paid":        model.TriggerShopifyOrderPaid,
	"woocommerce/created": model.TriggerWooOrderCreated,
}

// OnOrder schedules an Order confirmation, in the transaction that records the
// delivery. The run holds the rendered values, so a retry needs no store data.
func (s *Service) OnOrder(ctx context.Context, tx pgx.Tx, ev model.OrderEvent) error {
	allowed, err := s.deps.Plans.HasAccess(ctx, ev.WorkspaceID, model.PlanResource)
	if err != nil || !allowed {
		return err
	}
	items, err := s.enabled(ctx, ev.WorkspaceID)
	if err != nil {
		return err
	}
	order, ok := items[model.KindOrder].(*model.OrderSettings)
	if !ok || order.TemplateID == nil || order.Trigger != triggers[ev.Provider+"/"+ev.Status] {
		return nil
	}
	n := newRun{
		workspaceID: ev.WorkspaceID, kind: model.KindOrder, at: s.now(), immediate: true,
		dedupe:  "order:" + ev.Provider + ":" + ev.OrderID + ":" + ev.Status,
		context: model.RunContext{TemplateID: order.TemplateID, Params: orderParams(order.Variables, ev)},
	}
	contactID, err := s.deps.Contacts.FindOrCreateByPhone(ctx, ev.WorkspaceID, ev.Phone, strings.TrimSpace(ev.FirstName+" "+ev.LastName))
	switch {
	case errors.Is(err, model.ErrInvalidPhone):
		n.skipped, n.context = model.ReasonNoPhone, model.RunContext{}
	case err != nil:
		return fmt.Errorf("order contact: %w", err)
	default:
		n.contactID = &contactID
	}
	return s.addRun(ctx, tx, repository.NewRepository(tx), n)
}

func orderParams(vars []model.Variable, ev model.OrderEvent) []string {
	out := make([]string, len(vars))
	for i, v := range vars {
		switch v.Source {
		case model.SourceCustomerFirstName:
			out[i] = ev.FirstName
		case model.SourceOrderNumber:
			out[i] = ev.Number
		case model.SourceOrderTotal:
			out[i] = ev.Total
		case model.SourceTrackingURL:
			out[i] = ev.StatusURL
		case model.SourceCustom:
			out[i] = v.Custom
		}
	}
	return out
}

// RedactCustomer erases what runs hold about one customer.
func (s *Service) RedactCustomer(ctx context.Context, workspaceID uuid.UUID, phone string) error {
	id, ok, err := s.deps.Contacts.ContactIDByPhone(ctx, workspaceID, phone)
	if err != nil || !ok {
		return err
	}
	if err := repository.NewRepository(s.pool).RedactContactRuns(ctx, gen.RedactContactRunsParams{WorkspaceID: workspaceID, ContactID: &id}); err != nil {
		return fmt.Errorf("redact contact runs: %w", err)
	}
	return nil
}

// RedactStore erases what runs hold from store events.
func (s *Service) RedactStore(ctx context.Context, workspaceID uuid.UUID) error {
	if err := repository.NewRepository(s.pool).RedactStoreRuns(ctx, workspaceID); err != nil {
		return fmt.Errorf("redact store runs: %w", err)
	}
	return nil
}
