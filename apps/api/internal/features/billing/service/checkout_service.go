package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// Track T3: checkout sessions.

// errCheckoutReplay rolls back a checkout whose Idempotency-Key another request just used.
var errCheckoutReplay = errors.New("checkout: idempotency key already used")

// CreateCheckout starts a checkout for a price: a subscription (purpose new),
// or a lifetime deal in full or in installments (purpose ltd, which creates
// the pending order through CreateOrder). It applies and reserves a discount
// code, offers any enabled provider that has a price (Q10 deferred), and calls
// the gateway. The same IdempotencyKey returns the first session.
//
// A repeat returns the stored session without a client secret or URL: the
// provider gave those to the first response, and they are never stored.
func (s *Service) CreateCheckout(ctx context.Context, workspaceID, userID uuid.UUID, in model.CheckoutInput) (model.StartedCheckout, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return model.StartedCheckout{}, idempotencyKeyMissing()
	}
	same := func(c model.CheckoutSession) bool {
		return c.PlanPriceID == in.PlanPriceID && equalIDs(c.InstallmentOfferID, in.InstallmentOfferID)
	}
	r := repository.NewRepository(s.pool)
	if prev, ok, err := s.replayCheckout(ctx, r, workspaceID, key, same); err != nil || ok {
		return prev, err
	}

	price, err := s.loadPrice(ctx, r, in.PlanPriceID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	if !price.IsActive {
		return model.StartedCheckout{}, model.ErrPriceNotFound
	}
	var offer *model.InstallmentOffer
	if in.InstallmentOfferID != nil {
		if price.BillingType != model.BillingOneTime {
			return model.StartedCheckout{}, model.ErrOfferNotFound
		}
		o, err := s.InstallmentOffer(ctx, *in.InstallmentOfferID)
		if err != nil {
			return model.StartedCheckout{}, err
		}
		if o.PlanPriceID != price.ID {
			return model.StartedCheckout{}, model.ErrOfferNotFound
		}
		offer = &o
	}
	provider, collection, err := s.pickProvider(price, offer, in.Provider)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	g, err := s.gateway(provider)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	profile, err := s.Profile(ctx, workspaceID)
	if err != nil {
		return model.StartedCheckout{}, err
	}

	c := checkoutStart{
		gateway: g, profile: profile, matches: same, returnURL: in.ReturnURL,
		params: gen.CreateCheckoutSessionParams{
			WorkspaceID: workspaceID, PlanPriceID: price.ID, InstallmentOfferID: in.InstallmentOfferID,
			Provider: string(provider), Currency: price.Currency, IdempotencyKey: key, CreatedBy: optionalID(userID),
		},
	}
	if price.BillingType == model.BillingRecurring {
		err = s.prepareSubscription(ctx, r, &c, price, in.DiscountCode)
	} else {
		err = s.prepareOrder(ctx, r, &c, price, offer, collection, in.DiscountCode)
	}
	if err != nil {
		return model.StartedCheckout{}, err
	}
	return s.openCheckout(ctx, workspaceID, c)
}

// prepareSubscription fills in a checkout for a recurring price (purpose new).
func (s *Service) prepareSubscription(ctx context.Context, r repository.Repository, c *checkoutStart, price loadedPrice, code string) error {
	ws := c.params.WorkspaceID
	if _, err := r.GetLiveSubscription(ctx, ws); err == nil {
		return model.ErrAlreadySubscribed
	} else if !repository.IsNoRows(err) {
		return fmt.Errorf("get live subscription: %w", err)
	}
	if _, err := r.GetLiveOrder(ctx, ws); err == nil {
		return model.ErrHasLifetimeDeal
	} else if !repository.IsNoRows(err) {
		return fmt.Errorf("get live order: %w", err)
	}
	caps := c.gateway.Capabilities()
	if !caps.RenewsSubscriptions {
		// Relay-renewed (app) subscriptions come with SSLCommerz.
		return model.ErrNoProviderForPrice
	}
	pp, err := externalPriceID(ctx, r, price.ID, nil, c.gateway.Provider())
	if err != nil {
		return err
	}
	if pp.ExternalPriceID == nil {
		return model.ErrNoProviderForPrice
	}
	plan, err := r.GetPlan(ctx, price.PlanID)
	if err != nil {
		return fmt.Errorf("get plan: %w", err)
	}

	c.params.Purpose = model.PurposeNew
	c.price = &price
	c.req = payment.CheckoutRequest{
		Mode: payment.ModeSubscription, ExternalPriceID: *pp.ExternalPriceID, Currency: price.Currency,
		Description: fmt.Sprintf("%s, %sly", price.PlanName, price.Interval),
	}
	if code != "" {
		d, err := s.ApplyDiscount(ctx, ws, code, price.Price, nil)
		if err != nil {
			return err
		}
		if !caps.Coupons {
			// The provider charges the renewals, so it must carry the discount.
			return model.ErrDiscountWrongPrice
		}
		if c.req.ExternalCouponID, err = s.providerCoupon(ctx, c.gateway, d.Discount); err != nil {
			return err
		}
		c.discount = &d
		c.params.DiscountID, c.params.DiscountMinor = &d.Discount.ID, d.DiscountMinor
	}
	c.params.AmountMinor = price.AmountMinor - c.params.DiscountMinor
	if plan.TrialRequiresCard && plan.TrialDays > 0 {
		// A trial with a card starts in trialing and charges nothing now (Q5).
		// A workspace gets one trial; after it, the first period is paid.
		had, err := r.HasHadTrial(ctx, ws)
		if err != nil {
			return fmt.Errorf("has had trial: %w", err)
		}
		if !had {
			c.req.TrialDays, c.params.AmountMinor = int(plan.TrialDays), 0
		}
	}
	c.req.AmountMinor = c.params.AmountMinor
	return nil
}

// prepareOrder fills in a checkout for a one-time price, in full or through an
// installment offer (purpose ltd). Track T4's QuoteOrder works out the money.
func (s *Service) prepareOrder(ctx context.Context, r repository.Repository, c *checkoutStart, price loadedPrice, offer *model.InstallmentOffer, collection, code string) error {
	in := model.QuoteInput{PlanPriceID: price.ID, DiscountCode: code}
	if offer != nil {
		in.InstallmentOfferID = &offer.ID
	}
	q, err := s.QuoteOrder(ctx, c.params.WorkspaceID, in)
	if err != nil {
		return err
	}
	c.params.Purpose = model.PurposeLTD
	c.params.AmountMinor, c.params.Currency, c.params.DiscountMinor = q.DueNowMinor, q.Currency, q.DiscountMinor
	if q.Discount != nil {
		c.discount = q.Discount
		c.params.DiscountID = &q.Discount.Discount.ID
	}
	c.quote = &q
	product, err := oneTimeProduct(ctx, r, price.ID, c.gateway.Provider())
	if err != nil {
		return err
	}
	c.req = payment.CheckoutRequest{
		Mode: payment.ModePayment, ExternalPriceID: product, AmountMinor: q.DueNowMinor, Currency: q.Currency,
		Description: price.PlanName + ", lifetime",
	}
	if offer == nil {
		return nil
	}
	c.req.Description = fmt.Sprintf("%s, lifetime in %d payments", price.PlanName, offer.Count)
	if collection != model.CollectionProviderSchedule {
		return nil // app_link and provider_financed take the first payment as a one-off
	}
	if !c.gateway.Capabilities().Installments {
		return model.ErrNoProviderForPrice
	}
	pp, err := externalPriceID(ctx, r, price.ID, &offer.ID, c.gateway.Provider())
	if err != nil {
		return err
	}
	if pp.ExternalPriceID == nil {
		return model.ErrNoProviderForPrice
	}
	c.req.Mode, c.req.ExternalPriceID = payment.ModeInstallments, *pp.ExternalPriceID
	for _, p := range q.Schedule {
		c.req.Schedule = append(c.req.Schedule, p.AmountMinor)
	}
	return nil
}

// StartPaymentCheckout starts a one-off payment on an existing order: an
// app_link installment or a defaulted order's balance (purpose installment).
// Track T4's PayInstallment and ResumeOrder call it.
//
// InstallmentID is required: checkout_sessions allows purpose installment only
// with an installment (a balance payment names the first open one).
func (s *Service) StartPaymentCheckout(ctx context.Context, workspaceID, userID uuid.UUID, in model.PaymentCheckoutInput) (model.StartedCheckout, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return model.StartedCheckout{}, idempotencyKeyMissing()
	}
	if in.InstallmentID == nil || in.AmountMinor <= 0 {
		return model.StartedCheckout{}, model.ErrInvalidInput
	}
	same := func(c model.CheckoutSession) bool {
		return equalIDs(c.OrderID, &in.OrderID) && equalIDs(c.InstallmentID, in.InstallmentID)
	}
	r := repository.NewRepository(s.pool)
	if prev, ok, err := s.replayCheckout(ctx, r, workspaceID, key, same); err != nil || ok {
		return prev, err
	}
	order, err := r.GetOrder(ctx, gen.GetOrderParams{WorkspaceID: workspaceID, OrderID: in.OrderID})
	if repository.IsNoRows(err) {
		return model.StartedCheckout{}, model.ErrNoOrder
	}
	if err != nil {
		return model.StartedCheckout{}, fmt.Errorf("get order: %w", err)
	}
	provider := in.Provider
	if provider == "" {
		provider = payment.Provider(order.Provider)
	}
	g, err := s.gateway(provider)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	profile, err := s.Profile(ctx, workspaceID)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	currency := in.Currency
	if currency == "" {
		currency = order.Currency
	}
	product, err := oneTimeProduct(ctx, r, order.PlanPriceID, provider)
	if err != nil {
		return model.StartedCheckout{}, err
	}
	return s.openCheckout(ctx, workspaceID, checkoutStart{
		gateway: g, profile: profile, matches: same, returnURL: in.ReturnURL,
		params: gen.CreateCheckoutSessionParams{
			WorkspaceID: workspaceID, PlanPriceID: order.PlanPriceID, InstallmentOfferID: order.InstallmentOfferID,
			Purpose: model.PurposeInstallment, OrderID: &order.ID, InstallmentID: in.InstallmentID,
			Provider: string(provider), AmountMinor: in.AmountMinor, Currency: currency, IdempotencyKey: key,
			CreatedBy: optionalID(userID),
		},
		req: payment.CheckoutRequest{
			Mode: payment.ModePayment, ExternalPriceID: product, AmountMinor: in.AmountMinor, Currency: currency,
			Description: in.Description,
		},
	})
}

// checkoutStart is everything openCheckout needs to store and start one checkout.
type checkoutStart struct {
	gateway payment.Gateway
	// params is the session row; openCheckout sets OrderID (for a quote) and ExpiresAt.
	params   gen.CreateCheckoutSessionParams
	discount *model.AppliedDiscount
	// quote creates the pending order through CreateOrder (purpose ltd).
	quote *model.OrderQuote
	// price is set for a subscription, which is stored as incomplete when the provider creates it at once.
	price     *loadedPrice
	req       payment.CheckoutRequest
	profile   model.Profile
	returnURL string
	// matches says whether a session found under the same Idempotency-Key is this checkout.
	matches func(model.CheckoutSession) bool
}

// openCheckout stores the session (with its order and discount reservation)
// in one transaction, then asks the provider to start it. If the provider
// refuses, the session fails and the discount is released.
func (s *Service) openCheckout(ctx context.Context, workspaceID uuid.UUID, c checkoutStart) (model.StartedCheckout, error) {
	c.params.ExpiresAt = s.now().Add(s.cfg.CheckoutTTL)
	var (
		session  model.CheckoutSession
		customer gen.EnsureBillingCustomerRow
	)
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		var err error
		if customer, err = r.EnsureBillingCustomer(ctx, gen.EnsureBillingCustomerParams{
			WorkspaceID: workspaceID, Provider: c.params.Provider,
		}); err != nil {
			return fmt.Errorf("ensure billing customer: %w", err)
		}
		if c.quote != nil {
			o, err := s.CreateOrder(ctx, tx, workspaceID, model.NewOrder{
				Provider: c.gateway.Provider(), BillingCustomerID: customer.ID, Quote: *c.quote,
			})
			if err != nil {
				return err
			}
			c.params.OrderID = &o.ID
		}
		row, err := r.CreateCheckoutSession(ctx, c.params)
		if repository.IsNoRows(err) {
			return errCheckoutReplay
		}
		if err != nil {
			return fmt.Errorf("create checkout session: %w", err)
		}
		session = toCheckoutSession(gen.GetCheckoutSessionRow(row))
		if c.discount != nil {
			return s.ReserveDiscount(ctx, tx, workspaceID, session.ID, *c.discount)
		}
		return nil
	})
	if errors.Is(err, errCheckoutReplay) {
		prev, _, err := s.replayCheckout(ctx, repository.NewRepository(s.pool), workspaceID, c.params.IdempotencyKey, c.matches)
		return prev, err
	}
	if err != nil {
		return model.StartedCheckout{}, err
	}

	req := c.req
	req.Reference = session.ID
	req.IdempotencyKey = "checkout:" + session.ID.String()
	req.Customer = toCustomer(workspaceID, deref(customer.ExternalCustomerID), c.profile)
	req.ReturnURL = c.returnURL
	if req.ReturnURL == "" {
		req.ReturnURL = strings.TrimRight(s.cfg.AppBaseURL, "/") + "/billing"
	}
	started, err := c.gateway.CreateCheckout(ctx, req)
	if err != nil {
		if ferr := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
			return s.FailCheckout(ctx, tx, workspaceID, session.ID, model.CheckoutFailed)
		}); ferr != nil {
			s.logger().Error("fail checkout after provider error", "checkout_session_id", session.ID, "err", ferr)
		}
		return model.StartedCheckout{}, s.gatewayErr("create checkout", err)
	}

	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if started.ExternalSessionID != "" {
			if _, err := r.SetCheckoutSessionExternalID(ctx, gen.SetCheckoutSessionExternalIDParams{
				ExternalSessionID: &started.ExternalSessionID, WorkspaceID: workspaceID, CheckoutSessionID: session.ID,
			}); err != nil {
				return fmt.Errorf("set checkout external id: %w", err)
			}
			session.ExternalSessionID = &started.ExternalSessionID
		}
		// Keep the session open as long as the provider takes payment for it.
		if started.ExpiresAt.After(c.params.ExpiresAt) {
			if _, err := r.ExtendCheckoutSession(ctx, gen.ExtendCheckoutSessionParams{
				ExpiresAt: started.ExpiresAt, WorkspaceID: workspaceID, CheckoutSessionID: session.ID,
			}); err != nil {
				return fmt.Errorf("extend checkout session: %w", err)
			}
			session.ExpiresAt = started.ExpiresAt
		}
		if err := setCustomerExternalID(ctx, r, workspaceID, customer.ID, started.ExternalCustomerID); err != nil {
			return err
		}
		if started.ExternalSubscriptionID != "" && c.price != nil {
			id, err := r.CreateSubscription(ctx, gen.CreateSubscriptionParams{
				WorkspaceID: workspaceID, PlanPriceID: c.price.ID, BillingCustomerID: customer.ID,
				Provider: c.params.Provider, ExternalSubscriptionID: &started.ExternalSubscriptionID,
				RenewalMode: model.RenewalProvider, Status: model.SubscriptionIncomplete, DiscountID: c.params.DiscountID,
			})
			if err != nil {
				return fmt.Errorf("create subscription: %w", err)
			}
			if _, err := r.LinkCheckoutSession(ctx, gen.LinkCheckoutSessionParams{
				SubscriptionID: &id, WorkspaceID: workspaceID, CheckoutSessionID: session.ID,
			}); err != nil {
				return fmt.Errorf("link checkout session: %w", err)
			}
			session.SubscriptionID = &id
		}
		if started.ExternalScheduleID != "" && c.params.OrderID != nil {
			if _, err := r.SetOrderExternalRef(ctx, gen.SetOrderExternalRefParams{
				ExternalRef: &started.ExternalScheduleID, WorkspaceID: workspaceID, OrderID: *c.params.OrderID,
			}); err != nil {
				return fmt.Errorf("set order external ref: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		// The provider's checkout stands: its webhook names our session, which completes it.
		return model.StartedCheckout{}, err
	}
	return model.StartedCheckout{
		Session: session, ClientSecret: started.ClientSecret, URL: started.URL,
		PublishableKey: c.gateway.Capabilities().PublishableKey,
	}, nil
}

// replayCheckout returns the session stored under key, if any. A session for
// something else is model.ErrIdempotencyKeyReused.
func (s *Service) replayCheckout(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, key string, matches func(model.CheckoutSession) bool) (model.StartedCheckout, bool, error) {
	row, err := r.GetCheckoutSessionByIdempotencyKey(ctx, gen.GetCheckoutSessionByIdempotencyKeyParams{
		WorkspaceID: workspaceID, IdempotencyKey: key,
	})
	if repository.IsNoRows(err) {
		return model.StartedCheckout{}, false, nil
	}
	if err != nil {
		return model.StartedCheckout{}, false, fmt.Errorf("get checkout by idempotency key: %w", err)
	}
	session := toCheckoutSession(gen.GetCheckoutSessionRow(row))
	if !matches(session) {
		return model.StartedCheckout{}, false, model.ErrIdempotencyKeyReused
	}
	out := model.StartedCheckout{Session: session}
	if g, ok := s.payments.Get(session.Provider); ok {
		out.PublishableKey = g.Capabilities().PublishableKey
	}
	return out, true, nil
}

// pickProvider is the provider to sell price (or offer) through: want if set,
// else the first enabled provider that sells it. It returns the offer's
// collection for that provider.
func (s *Service) pickProvider(price loadedPrice, offer *model.InstallmentOffer, want payment.Provider) (payment.Provider, string, error) {
	if want != "" {
		if _, ok := s.payments.Get(want); !ok {
			return "", "", model.ErrProviderNotAvailable
		}
	}
	if offer != nil {
		for _, op := range offer.Providers {
			if want == "" || op.Provider == want {
				return op.Provider, op.Collection, nil
			}
		}
		return "", "", model.ErrNoProviderForPrice
	}
	for _, p := range price.Providers {
		if want == "" || p == want {
			return p, "", nil
		}
	}
	return "", "", model.ErrNoProviderForPrice
}

// CheckoutSession returns one of the workspace's checkout sessions, so the
// browser can see it complete. model.ErrCheckoutNotFound if there is none.
func (s *Service) CheckoutSession(ctx context.Context, workspaceID, sessionID uuid.UUID) (model.CheckoutSession, error) {
	row, err := repository.NewRepository(s.pool).GetCheckoutSession(ctx, gen.GetCheckoutSessionParams{
		WorkspaceID: workspaceID, CheckoutSessionID: sessionID,
	})
	if repository.IsNoRows(err) {
		return model.CheckoutSession{}, model.ErrCheckoutNotFound
	}
	if err != nil {
		return model.CheckoutSession{}, fmt.Errorf("get checkout session: %w", err)
	}
	return toCheckoutSession(row), nil
}

// CompleteTestCheckout pays a pending checkout on a test gateway the way its
// provider would: the fake signs checkout.completed, and the event goes through
// the webhook path, so the session completes once the job has run. Only fake
// gateways (PAYMENT_PROVIDERS=stripe:fake, development only) support it; for
// any other the session is model.ErrCheckoutNotFound.
func (s *Service) CompleteTestCheckout(ctx context.Context, workspaceID, sessionID uuid.UUID) (model.CheckoutSession, error) {
	c, err := s.CheckoutSession(ctx, workspaceID, sessionID)
	if err != nil {
		return model.CheckoutSession{}, err
	}
	g, err := s.gateway(c.Provider)
	if err != nil {
		return model.CheckoutSession{}, err
	}
	fake, ok := g.(*payment.Fake)
	if !ok || c.ExternalSessionID == nil {
		return model.CheckoutSession{}, model.ErrCheckoutNotFound
	}
	if c.Status != model.CheckoutPending {
		return c, nil
	}
	ev, err := fake.CompleteCheckout(*c.ExternalSessionID)
	if err != nil {
		return model.CheckoutSession{}, fmt.Errorf("complete test checkout: %w", err)
	}
	header, body, err := fake.Webhook(ev)
	if err != nil {
		return model.CheckoutSession{}, fmt.Errorf("sign test webhook: %w", err)
	}
	if err := s.ReceiveWebhook(ctx, string(c.Provider), header, body); err != nil {
		return model.CheckoutSession{}, err
	}
	return c, nil
}

// CompleteCheckout applies checkout.completed in tx: the session becomes
// completed, its discount redeemed, and then by purpose a subscription is
// activated with its grant (T3) or an order is paid through
// CompleteOrderCheckout (T4). A session that already finished is a no-op.
func (s *Service) CompleteCheckout(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, ev payment.Event) error {
	r := repository.NewRepository(tx)
	row, err := r.LockCheckoutSession(ctx, gen.LockCheckoutSessionParams{WorkspaceID: workspaceID, CheckoutSessionID: sessionID})
	if repository.IsNoRows(err) {
		return model.ErrCheckoutNotFound
	}
	if err != nil {
		return fmt.Errorf("lock checkout session: %w", err)
	}
	session := toCheckoutSession(gen.GetCheckoutSessionRow(row))
	if ev.Payment == nil && session.Status != model.CheckoutCompleted {
		return fmt.Errorf("checkout %s completed without a payment", session.ID)
	}
	switch session.Status {
	case model.CheckoutCompleted:
		return nil // a replay
	case model.CheckoutFailed, model.CheckoutExpired:
		// The customer paid after we gave up (a retried card, a tab left open):
		// the money is real, so the checkout completes, with its discount back.
		if err := s.reopenCheckout(ctx, r, session); err != nil {
			return err
		}
	}
	if _, err := r.FinishCheckoutSession(ctx, gen.FinishCheckoutSessionParams{
		Status: model.CheckoutCompleted, WorkspaceID: workspaceID, CheckoutSessionID: session.ID,
	}); err != nil {
		return fmt.Errorf("complete checkout session: %w", err)
	}
	switch session.Purpose {
	case model.PurposeNew:
		subID, err := s.activateSubscription(ctx, tx, session, ev)
		if err != nil {
			return err
		}
		return s.RedeemDiscount(ctx, tx, workspaceID, session.ID, &subID, nil)
	case model.PurposeLTD, model.PurposeInstallment:
		if err := s.CompleteOrderCheckout(ctx, tx, workspaceID, session, *ev.Payment); err != nil {
			return err
		}
		return s.RedeemDiscount(ctx, tx, workspaceID, session.ID, nil, session.OrderID)
	default:
		return fmt.Errorf("checkout %s: purpose %q has no completion", session.ID, session.Purpose)
	}
}

// reopenCheckout puts a failed or expired session back to pending in tx, and
// its discount reservation with it while the code has uses left.
func (s *Service) reopenCheckout(ctx context.Context, r repository.Repository, session model.CheckoutSession) error {
	s.logger().Info("late payment for a closed checkout", "checkout_session_id", session.ID, "status", session.Status)
	if _, err := r.ReopenCheckoutSession(ctx, gen.ReopenCheckoutSessionParams{
		WorkspaceID: session.WorkspaceID, CheckoutSessionID: session.ID,
	}); err != nil {
		return fmt.Errorf("reopen checkout session: %w", err)
	}
	if session.DiscountID == nil {
		return nil
	}
	n, err := r.ReclaimDiscount(ctx, gen.ReclaimDiscountParams{
		WorkspaceID: session.WorkspaceID, CheckoutSessionID: session.ID,
	})
	if err != nil {
		return fmt.Errorf("reclaim discount: %w", err)
	}
	if n == 0 {
		// Used up meanwhile: the customer keeps the price they paid, uncounted.
		s.logger().Warn("discount used up before a late payment", "checkout_session_id", session.ID)
	}
	return nil
}

// activateSubscription makes a paid purpose-new checkout's subscription live
// in tx, records its first invoice and gives the workspace the plan.
func (s *Service) activateSubscription(ctx context.Context, tx pgx.Tx, session model.CheckoutSession, ev payment.Event) (uuid.UUID, error) {
	r := repository.NewRepository(tx)
	ws := session.WorkspaceID
	p := *ev.Payment
	ps := payment.Subscription{ExternalID: p.ExternalSubscriptionID, ExternalCustomerID: p.ExternalCustomerID}
	if ev.Subscription != nil {
		ps = *ev.Subscription
	}
	price, err := s.loadPrice(ctx, r, session.PlanPriceID)
	if err != nil {
		return uuid.Nil, err
	}
	customer, err := r.EnsureBillingCustomer(ctx, gen.EnsureBillingCustomerParams{WorkspaceID: ws, Provider: string(session.Provider)})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ensure billing customer: %w", err)
	}
	extCustomer := p.ExternalCustomerID
	if extCustomer == "" {
		extCustomer = ps.ExternalCustomerID
	}
	if err := setCustomerExternalID(ctx, r, ws, customer.ID, extCustomer); err != nil {
		return uuid.Nil, err
	}

	now := s.now()
	status := model.SubscriptionActive
	if ps.Status == payment.SubscriptionTrialing || ps.Status == payment.SubscriptionPastDue {
		status = string(ps.Status)
	}
	start, end := ps.CurrentPeriodStart, ps.CurrentPeriodEnd
	if start == nil || end == nil {
		start, end = p.PeriodStart, p.PeriodEnd
	}
	var discountEnds *time.Time
	if session.DiscountID != nil {
		d, err := r.GetDiscount(ctx, *session.DiscountID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("get discount: %w", err)
		}
		from := now
		if start != nil {
			from = *start
		}
		discountEnds = discountEndsAt(toDiscount(d), from, end)
	}

	var subID uuid.UUID
	if session.SubscriptionID != nil {
		subID = *session.SubscriptionID
		locked, err := r.LockSubscription(ctx, gen.LockSubscriptionParams{WorkspaceID: ws, SubscriptionID: subID})
		if err != nil {
			return uuid.Nil, fmt.Errorf("lock subscription: %w", err)
		}
		sub := toSubscription(gen.GetSubscriptionRow(locked))
		sub.Status, sub.ProviderStatus = status, nilIfEmpty(ps.ProviderStatus)
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CancelAtPeriodEnd = start, end, ps.CancelAtPeriodEnd
		if err := syncPeriod(ctx, r, sub); err != nil {
			return uuid.Nil, liveConflict(err, session.ID)
		}
		if sub.ExternalSubscriptionID == nil && ps.ExternalID != "" {
			if _, err := r.SetSubscriptionExternalID(ctx, gen.SetSubscriptionExternalIDParams{
				ExternalSubscriptionID: &ps.ExternalID, WorkspaceID: ws, SubscriptionID: subID,
			}); err != nil {
				return uuid.Nil, fmt.Errorf("set subscription external id: %w", err)
			}
		}
		if session.DiscountID != nil {
			if _, err := r.SetSubscriptionDiscount(ctx, gen.SetSubscriptionDiscountParams{
				DiscountID: session.DiscountID, DiscountEndsAt: discountEnds, WorkspaceID: ws, SubscriptionID: subID,
			}); err != nil {
				return uuid.Nil, fmt.Errorf("set subscription discount: %w", err)
			}
		}
	} else {
		subID, err = r.CreateSubscription(ctx, gen.CreateSubscriptionParams{
			WorkspaceID: ws, PlanPriceID: session.PlanPriceID, BillingCustomerID: customer.ID,
			Provider: string(session.Provider), ExternalSubscriptionID: nilIfEmpty(ps.ExternalID),
			RenewalMode: model.RenewalProvider, Status: status, ProviderStatus: nilIfEmpty(ps.ProviderStatus),
			CurrentPeriodStart: start, CurrentPeriodEnd: end, DiscountID: session.DiscountID, DiscountEndsAt: discountEnds,
		})
		if err != nil {
			return uuid.Nil, liveConflict(err, session.ID)
		}
		if _, err := r.LinkCheckoutSession(ctx, gen.LinkCheckoutSessionParams{
			SubscriptionID: &subID, WorkspaceID: ws, CheckoutSessionID: session.ID,
		}); err != nil {
			return uuid.Nil, fmt.Errorf("link checkout session: %w", err)
		}
	}

	billTo, err := s.BillTo(ctx, tx, ws)
	if err != nil {
		return uuid.Nil, err
	}
	discountMinor := p.DiscountMinor
	if discountMinor == 0 {
		// Relay worked the discount into the amount charged.
		discountMinor = session.DiscountMinor
	}
	if _, err := createInvoice(ctx, r, ws, invoiceInput{
		Provider: session.Provider, SubscriptionID: &subID, CheckoutSessionID: &session.ID, Payment: p, Paid: true,
		BillTo: billTo, DiscountID: session.DiscountID, DiscountMinor: discountMinor, Now: now,
	}); err != nil {
		return uuid.Nil, err
	}
	if err := s.ensureSubscriptionGrant(ctx, tx, ws, subID, price.PlanID); err != nil {
		return uuid.Nil, err
	}
	return subID, nil
}

// liveConflict explains a second live subscription: two checkouts were paid at once.
func liveConflict(err error, sessionID uuid.UUID) error {
	if repository.IsUnique(err, repository.LiveSubscriptionIndex) {
		return fmt.Errorf("checkout %s paid while another subscription is live; refund it: %w", sessionID, err)
	}
	return fmt.Errorf("save subscription: %w", err)
}

// FailCheckout ends a pending session as model.CheckoutFailed or
// model.CheckoutExpired in tx and releases its reserved discount.
func (s *Service) FailCheckout(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, status string) error {
	if status != model.CheckoutFailed && status != model.CheckoutExpired {
		return model.ErrInvalidInput
	}
	r := repository.NewRepository(tx)
	n, err := r.FinishCheckoutSession(ctx, gen.FinishCheckoutSessionParams{
		Status: status, WorkspaceID: workspaceID, CheckoutSessionID: sessionID,
	})
	if err != nil {
		return fmt.Errorf("finish checkout session: %w", err)
	}
	if n == 0 {
		// Already finished, or not this workspace's.
		_, err := r.GetCheckoutSession(ctx, gen.GetCheckoutSessionParams{WorkspaceID: workspaceID, CheckoutSessionID: sessionID})
		if repository.IsNoRows(err) {
			return model.ErrCheckoutNotFound
		}
		if err != nil {
			return fmt.Errorf("get checkout session: %w", err)
		}
		return nil
	}
	return s.ReleaseDiscount(ctx, tx, workspaceID, sessionID)
}

// ExpireCheckouts expires pending sessions past expires_at, releasing their
// discounts, and returns how many. Run by the ExpireCheckouts worker.
func (s *Service) ExpireCheckouts(ctx context.Context) (int, error) {
	expired := 0
	for {
		rows, err := repository.NewRepository(s.pool).ListExpiredCheckoutSessions(ctx, gen.ListExpiredCheckoutSessionsParams{
			Now: s.now(), MaxRows: jobBatch,
		})
		if err != nil {
			return expired, fmt.Errorf("list expired checkouts: %w", err)
		}
		for _, row := range rows {
			if err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
				return s.FailCheckout(ctx, tx, row.WorkspaceID, row.ID, model.CheckoutExpired)
			}); err != nil {
				return expired, err
			}
			expired++
		}
		if len(rows) < jobBatch {
			return expired, nil
		}
	}
}

// invoiceInput is one payment to record as an invoice.
type invoiceInput struct {
	Provider          payment.Provider
	SubscriptionID    *uuid.UUID
	OrderID           *uuid.UUID
	CheckoutSessionID *uuid.UUID
	Payment           payment.Payment
	Paid              bool
	BillTo            model.BillTo
	// DiscountID is our discount; without one the invoice shows no discount line.
	DiscountID *uuid.UUID
	// DiscountMinor overrides Payment.DiscountMinor when set.
	DiscountMinor int64
	Now           time.Time
}

// createInvoice records a payment. False means the provider's payment is
// already recorded (a replayed webhook).
func createInvoice(ctx context.Context, r repository.Repository, workspaceID uuid.UUID, in invoiceInput) (bool, error) {
	billTo, err := json.Marshal(in.BillTo)
	if err != nil {
		return false, fmt.Errorf("encode bill_to: %w", err)
	}
	p := in.Payment
	discount := in.DiscountMinor
	if discount == 0 {
		discount = p.DiscountMinor
	}
	discountID := in.DiscountID
	if discountID == nil || discount <= 0 {
		discount, discountID = 0, nil
	}
	status, paidAt := model.InvoiceFailed, (*time.Time)(nil)
	if in.Paid {
		status, paidAt = model.InvoicePaid, p.PaidAt
		if paidAt == nil {
			now := in.Now
			if now.IsZero() {
				now = time.Now().UTC()
			}
			paidAt = &now
		}
	}
	_, err = r.CreateInvoice(ctx, gen.CreateInvoiceParams{
		WorkspaceID: workspaceID, Number: nilIfEmpty(p.InvoiceNumber), SubscriptionID: in.SubscriptionID,
		OrderID: in.OrderID, CheckoutSessionID: in.CheckoutSessionID, Provider: string(in.Provider),
		ExternalPaymentID: nilIfEmpty(p.ExternalID), Status: status,
		// total = subtotal - discount + tax always holds, whatever the provider rounded.
		SubtotalMinor: p.TotalMinor - p.TaxMinor + discount, TaxMinor: p.TaxMinor, TotalMinor: p.TotalMinor,
		Currency: p.Currency, PeriodStart: p.PeriodStart, PeriodEnd: p.PeriodEnd, BillTo: billTo,
		DiscountID: discountID, DiscountMinor: discount, ReceiptUrl: nilIfEmpty(p.ReceiptURL), PaidAt: paidAt,
	})
	if repository.IsNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("create invoice: %w", err)
	}
	return true, nil
}

// setCustomerExternalID stores the provider's customer ID the first time it is seen.
func setCustomerExternalID(ctx context.Context, r repository.Repository, workspaceID, customerID uuid.UUID, external string) error {
	if external == "" {
		return nil
	}
	if _, err := r.SetBillingCustomerExternalID(ctx, gen.SetBillingCustomerExternalIDParams{
		ExternalCustomerID: &external, WorkspaceID: workspaceID, BillingCustomerID: customerID,
	}); err != nil {
		return fmt.Errorf("set customer external id: %w", err)
	}
	return nil
}

// gatewayErr turns a provider failure into the error the customer sees, and logs why.
func (s *Service) gatewayErr(op string, err error) error {
	var out error
	switch {
	case errors.Is(err, payment.ErrDeclined):
		out = model.ErrPaymentProviderDeclined
	case errors.Is(err, payment.ErrUnavailable):
		out = model.ErrPaymentProviderDown
	case errors.Is(err, payment.ErrUnsupported):
		out = model.ErrChangeNotSupported
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
	s.logger().Warn("payment provider refused", "op", op, "err", err)
	return out
}

// logger is the service's logger, or a silent one when none was given.
func (s *Service) logger() *slog.Logger {
	if s.log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.log
}

func toCustomer(workspaceID uuid.UUID, externalID string, p model.Profile) payment.Customer {
	return payment.Customer{
		ExternalID: externalID, WorkspaceID: workspaceID, Name: p.LegalName, Email: p.Email, Phone: deref(p.Phone),
		Address: payment.Address{
			Line1: deref(p.AddressLine1), Line2: deref(p.AddressLine2), City: deref(p.City), State: deref(p.State),
			PostalCode: deref(p.PostalCode), Country: deref(p.Country),
		},
		TaxIDType: deref(p.TaxIDType), TaxID: deref(p.TaxID),
	}
}

func toCheckoutSession(r gen.GetCheckoutSessionRow) model.CheckoutSession {
	return model.CheckoutSession{
		ID: r.ID, WorkspaceID: r.WorkspaceID, PlanPriceID: r.PlanPriceID, InstallmentOfferID: r.InstallmentOfferID,
		Purpose: r.Purpose, SubscriptionID: r.SubscriptionID, OrderID: r.OrderID, InstallmentID: r.InstallmentID,
		Provider: payment.Provider(r.Provider), ExternalSessionID: r.ExternalSessionID, Status: r.Status,
		AmountMinor: r.AmountMinor, Currency: r.Currency, DiscountID: r.DiscountID, DiscountMinor: r.DiscountMinor,
		CreatedBy: r.CreatedBy, ExpiresAt: r.ExpiresAt, CreatedAt: r.CreatedAt,
	}
}

func idempotencyKeyMissing() error {
	return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "Idempotency-Key", Message: "Send an Idempotency-Key header"})
}

// optionalID is nil for uuid.Nil (no user, such as a job), else &id.
func optionalID(id uuid.UUID) *uuid.UUID {
	if id == uuid.Nil {
		return nil
	}
	return &id
}

func equalIDs(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
