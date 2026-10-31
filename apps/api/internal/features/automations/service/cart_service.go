package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/repository"
)

const secondReminderAfter = 24 * time.Hour

func cartKey(token string) string { return "cart:" + token }

// OnCheckout starts or restarts a checkout's wait. Shopify has no abandoned
// event, so the reminder fires when a checkout stops changing. A checkout with
// no phone yet is left alone: a later update may bring one.
func (s *Service) OnCheckout(ctx context.Context, tx pgx.Tx, ev model.CheckoutEvent) error {
	allowed, err := s.deps.Plans.HasAccess(ctx, ev.WorkspaceID, model.PlanResource)
	if err != nil || !allowed {
		return err
	}
	items, err := s.enabled(ctx, ev.WorkspaceID)
	if err != nil {
		return err
	}
	cart, ok := items[model.KindCart].(*model.CartSettings)
	if !ok || cart.TemplateID == nil {
		return nil
	}
	contactID, err := s.deps.Contacts.FindOrCreateByPhone(ctx, ev.WorkspaceID, ev.Phone, strings.TrimSpace(ev.FirstName+" "+ev.LastName))
	if errors.Is(err, model.ErrInvalidPhone) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checkout contact: %w", err)
	}
	raw, err := json.Marshal(model.RunContext{
		TemplateID: cart.TemplateID, Params: cartParams(cart, ev), Consent: ev.Consent, Second: cart.SecondReminder,
	})
	if err != nil {
		return fmt.Errorf("encode run context: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("new run id: %w", err)
	}
	at := s.now().Add(time.Duration(cart.DelayMinutes) * time.Minute)
	if err := repository.NewRepository(tx).UpsertCartRun(ctx, gen.UpsertCartRunParams{
		ID: id, WorkspaceID: ev.WorkspaceID, ContactID: &contactID, DedupeKey: cartKey(ev.Token), NextAt: &at, Context: raw,
	}); err != nil {
		return fmt.Errorf("upsert cart run: %w", err)
	}
	return nil
}

func cartParams(cart *model.CartSettings, ev model.CheckoutEvent) []string {
	out := make([]string, len(cart.Variables))
	for i, v := range cart.Variables {
		switch v.Source {
		case model.SourceCustomerFirstName:
			out[i] = ev.FirstName
		case model.SourceCartTotal:
			out[i] = ev.Total
		case model.SourceCheckoutURL:
			out[i] = ev.CheckoutURL
		case model.SourceDiscountCode:
			out[i] = cart.DiscountCode
		case model.SourceCustom:
			out[i] = v.Custom
		}
	}
	return out
}

// closeCart ends a checkout's run when its order arrives: unsent, it cancels;
// after a reminder, the order counts as recovered.
func (s *Service) closeCart(ctx context.Context, tx pgx.Tx, ev model.OrderEvent) error {
	if ev.CheckoutToken == "" {
		return nil
	}
	r, key := repository.NewRepository(tx), cartKey(ev.CheckoutToken)
	if err := r.CancelCartRun(ctx, gen.CancelCartRunParams{WorkspaceID: ev.WorkspaceID, DedupeKey: key}); err != nil {
		return fmt.Errorf("cancel cart run: %w", err)
	}
	if err := r.RecoverCart(ctx, gen.RecoverCartParams{
		AmountMinor: &ev.AmountMinor, Currency: &ev.Currency, WorkspaceID: ev.WorkspaceID, DedupeKey: key,
	}); err != nil {
		return fmt.Errorf("recover cart: %w", err)
	}
	return nil
}
