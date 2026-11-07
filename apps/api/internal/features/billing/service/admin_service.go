package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// The staff admin tools (/v1/admin): build the catalog and move workspaces
// between plans by hand. Catalog rows are never edited: deactivate one and
// add its replacement, so what a customer bought stays as it was. A plan's
// limits are the exception (ADR 0008): they are live, and every edit is audited.

var planCode = regexp.MustCompile(`^[a-z][a-z0-9_]{1,40}$`)

// AdminCatalog returns every plan, price, installment offer and provider price, active or not.
func (s *Service) AdminCatalog(ctx context.Context) (model.AdminCatalog, error) {
	r := repository.NewRepository(s.pool)
	plans, err := r.AdminListPlans(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("list plans: %w", err)
	}
	prices, err := r.AdminListPlanPrices(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("list prices: %w", err)
	}
	offers, err := r.AdminListInstallmentOffers(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("list installment offers: %w", err)
	}
	links, err := r.AdminListProviderPrices(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("list provider prices: %w", err)
	}

	priceLinks := map[uuid.UUID][]model.AdminProviderPrice{}
	offerLinks := map[uuid.UUID][]model.AdminProviderPrice{}
	for _, l := range links {
		pp := model.AdminProviderPrice{
			ID: l.ID, Provider: l.Provider, ExternalPriceID: l.ExternalPriceID, Collection: l.Collection, IsActive: l.IsActive,
		}
		if l.InstallmentOfferID != nil {
			offerLinks[*l.InstallmentOfferID] = append(offerLinks[*l.InstallmentOfferID], pp)
		} else {
			priceLinks[l.PlanPriceID] = append(priceLinks[l.PlanPriceID], pp)
		}
	}
	priceOffers := map[uuid.UUID][]model.AdminOffer{}
	for _, o := range offers {
		priceOffers[o.PlanPriceID] = append(priceOffers[o.PlanPriceID], model.AdminOffer{
			ID: o.ID, Count: int(o.Count), TotalMinor: o.TotalMinor, GraceDays: int(o.GraceDays),
			MaxMissed: int(o.MaxMissed), IsActive: o.IsActive, ProviderPrices: offerLinks[o.ID],
		})
	}
	planPrices := map[uuid.UUID][]model.AdminPrice{}
	for _, p := range prices {
		planPrices[p.PlanID] = append(planPrices[p.PlanID], model.AdminPrice{
			ID: p.ID, BillingType: p.BillingType, Interval: deref(p.Interval), Currency: p.Currency,
			AmountMinor: p.AmountMinor, IsActive: p.IsActive, ProviderPrices: priceLinks[p.ID], Offers: priceOffers[p.ID],
		})
	}

	counts, err := r.AdminCountPlanWorkspaces(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("count plan workspaces: %w", err)
	}
	onPlan := make(map[uuid.UUID]int, len(counts))
	for _, c := range counts {
		onPlan[c.PlanID] = int(c.Workspaces)
	}
	onFree, err := r.AdminCountFreeWorkspaces(ctx)
	if err != nil {
		return model.AdminCatalog{}, fmt.Errorf("count free workspaces: %w", err)
	}

	out := model.AdminCatalog{Plans: make([]model.AdminPlan, 0, len(plans)), Providers: s.payments.Providers()}
	for _, p := range plans {
		limits, err := model.ParseLimits(p.Limits)
		if err != nil {
			return model.AdminCatalog{}, err
		}
		workspaces := onPlan[p.ID]
		if p.Code == model.FreePlan {
			workspaces = int(onFree)
		}
		out.Plans = append(out.Plans, model.AdminPlan{
			ID: p.ID, Code: p.Code, Version: int(p.Version), Name: p.Name, Description: p.Description,
			Features: p.Features, Recommended: p.Recommended, TrialDays: int(p.TrialDays),
			TrialRequiresCard: p.TrialRequiresCard, Limits: limits, IsActive: p.IsActive, SortOrder: int(p.SortOrder),
			Prices: planPrices[p.ID], Workspaces: workspaces,
		})
	}
	return out, nil
}

// Limits on what the pricing table shows about a plan (migration 00015).
const (
	maxPlanDescription = 200
	maxPlanFeatures    = 20
	maxPlanFeature     = 80
)

// CreatePlan adds a plan, or the next version of one whose code is already used.
func (s *Service) CreatePlan(ctx context.Context, in model.NewPlan) (uuid.UUID, error) {
	d, details := checkPlanDetails(in.PlanDetails, false)
	code := strings.ToLower(strings.TrimSpace(in.Code))
	if !planCode.MatchString(code) {
		details = append(details, apperr.Detail{Field: "code", Message: "Use 2 to 41 lowercase letters, digits or _, starting with a letter"})
	}
	if code == model.FreePlan {
		details = append(details, apperr.Detail{Field: "code", Message: "free is the plan with no grant; edit it instead"})
	}
	if len(details) > 0 {
		return uuid.Nil, model.ErrInvalidInput.WithDetails(details...)
	}
	limits, err := json.Marshal(d.Limits)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encode limits: %w", err)
	}
	var id uuid.UUID
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if in.PublishKey != nil {
			prev, err := r.GetPlanByPublishKey(ctx, in.PublishKey)
			switch {
			case err == nil && prev.Code == code:
				id = prev.ID
				return nil
			case err == nil:
				return model.ErrCatalogDuplicate
			case !repository.IsNoRows(err):
				return fmt.Errorf("get plan by publish key: %w", err)
			}
		}
		version, err := r.NextPlanVersion(ctx, code)
		if err != nil {
			return fmt.Errorf("next plan version: %w", err)
		}
		id, err = r.CreatePlan(ctx, gen.CreatePlanParams{
			Code: code, Version: version, Name: d.Name, Description: d.Description, Features: d.Features,
			Recommended: d.Recommended, TrialDays: int32(d.TrialDays), //nolint:gosec // range checked above
			TrialRequiresCard: d.TrialRequiresCard, Limits: limits, SortOrder: int32(d.SortOrder), //nolint:gosec // a small display order
			PublishKey: in.PublishKey,
		})
		return err
	})
	if errors.Is(err, model.ErrCatalogDuplicate) {
		return uuid.Nil, err
	}
	return id, catalogErr("create plan", err)
}

// UpdatePlan changes a plan version in place: everything but its code, version
// and prices. Its limits reach every workspace on it at once; a change to them
// needs a reason and is audited (ADR 0008).
func (s *Service) UpdatePlan(ctx context.Context, in model.PlanEdit) error {
	reason := strings.TrimSpace(in.Reason)
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		plan, err := r.LockPlanLimits(ctx, in.PlanID)
		if repository.IsNoRows(err) {
			return model.ErrPlanNotFound
		}
		if err != nil {
			return fmt.Errorf("lock plan: %w", err)
		}
		d, details := checkPlanDetails(in.PlanDetails, plan.Code == model.FreePlan)
		before, err := model.ParseLimits(plan.Limits)
		if err != nil {
			return fmt.Errorf("plan %s: %w", in.PlanID, err)
		}
		limitsChanged := !maps.Equal(before, d.Limits)
		if limitsChanged && reason == "" {
			details = append(details, apperr.Detail{Field: "reason", Message: "Say why the limits change, for the record"})
		}
		if len(details) > 0 {
			return model.ErrInvalidInput.WithDetails(details...)
		}
		after, err := json.Marshal(d.Limits)
		if err != nil {
			return fmt.Errorf("encode limits: %w", err)
		}
		if err := r.UpdatePlan(ctx, gen.UpdatePlanParams{
			PlanID: in.PlanID, Name: d.Name, Description: d.Description, Features: d.Features,
			Recommended: d.Recommended, TrialDays: int32(d.TrialDays), //nolint:gosec // range checked above
			TrialRequiresCard: d.TrialRequiresCard, Limits: after, SortOrder: int32(d.SortOrder), //nolint:gosec // a small display order
		}); err != nil {
			return fmt.Errorf("update plan: %w", err)
		}
		if !limitsChanged {
			return nil
		}
		return r.InsertLimitChange(ctx, gen.InsertLimitChangeParams{
			PlanID: &in.PlanID, ChangedBy: &in.ChangedBy, Before: plan.Limits, After: after, Reason: reason,
		})
	})
}

// checkPlanDetails trims what staff entered and lists what is wrong with it.
// The free plan is never on the pricing table and has no trial.
func checkPlanDetails(d model.PlanDetails, free bool) (model.PlanDetails, []apperr.Detail) {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" {
		bad("name", "A plan needs a name")
	}
	d.Description = strings.TrimSpace(d.Description)
	if len([]rune(d.Description)) > maxPlanDescription {
		bad("description", "Keep the description to 200 characters")
	}
	features := make([]string, 0, len(d.Features))
	for _, f := range d.Features {
		if f = strings.TrimSpace(f); f != "" {
			features = append(features, f)
		}
	}
	d.Features = features
	if len(features) > maxPlanFeatures {
		bad("features", "List at most 20 features")
	}
	for _, f := range features {
		if len([]rune(f)) > maxPlanFeature {
			bad("features", "Keep each feature to 80 characters")
			break
		}
	}
	if d.TrialDays < 0 || d.TrialDays > 90 {
		bad("trial_days", "A trial lasts 0 to 90 days")
	}
	if free && (d.TrialDays > 0 || d.TrialRequiresCard || d.Recommended) {
		bad("trial_days", "The free plan has no trial and isn't on the pricing table")
	}
	if d.Limits == nil {
		d.Limits = model.Limits{}
	}
	details = append(details, d.Limits.Check("limits")...)
	return d, details
}

// CreatePrice adds a price to a plan.
func (s *Service) CreatePrice(ctx context.Context, in model.NewPrice) (uuid.UUID, error) {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }
	params := gen.CreatePlanPriceParams{
		PlanID: in.PlanID, BillingType: in.BillingType, Currency: strings.ToUpper(strings.TrimSpace(in.Currency)),
		AmountMinor: in.AmountMinor,
	}
	switch in.BillingType {
	case model.BillingRecurring:
		if in.Interval != model.IntervalMonth && in.Interval != model.IntervalYear {
			bad("interval", "A recurring price is monthly or yearly")
		}
		params.Interval = &in.Interval
	case model.BillingOneTime:
		if in.Interval != "" {
			bad("interval", "A one-time price has no interval")
		}
	default:
		bad("billing_type", "Billing type is recurring or one_time")
	}
	if !currencyCode.MatchString(params.Currency) {
		bad("currency", "Use a three-letter currency code")
	}
	if in.AmountMinor < 0 {
		bad("amount_minor", "A price can't be negative")
	}
	if len(details) > 0 {
		return uuid.Nil, model.ErrInvalidInput.WithDetails(details...)
	}
	r := repository.NewRepository(s.pool)
	plan, err := r.GetPlan(ctx, in.PlanID)
	if repository.IsNoRows(err) {
		return uuid.Nil, model.ErrPlanNotFound
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("get plan: %w", err)
	}
	if plan.Code == model.FreePlan {
		return uuid.Nil, model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "plan_id", Message: "The free plan can't be sold"})
	}
	id, err := r.CreatePlanPrice(ctx, params)
	if repository.IsForeignKey(err) {
		return uuid.Nil, model.ErrPlanNotFound
	}
	if repository.IsDuplicate(err) {
		// The slot is taken. By this very price, it is a retried publish: return it.
		cur, getErr := r.GetActivePlanPriceInSlot(ctx, gen.GetActivePlanPriceInSlotParams{
			PlanID: params.PlanID, BillingType: params.BillingType, Interval: params.Interval, Currency: params.Currency,
		})
		if getErr == nil && cur.AmountMinor == params.AmountMinor {
			return cur.ID, nil
		}
	}
	return id, catalogErr("create price", err)
}

// CreateInstallmentOffer splits a one-time price into monthly payments.
func (s *Service) CreateInstallmentOffer(ctx context.Context, in model.NewInstallmentOffer) (uuid.UUID, error) {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }
	if in.Count < 2 || in.Count > 24 {
		bad("count", "2 to 24 installments")
	}
	if in.TotalMinor <= 0 {
		bad("total_minor", "The installments must add up to more than 0")
	}
	if in.GraceDays < 0 || in.GraceDays > 60 {
		bad("grace_days", "0 to 60 days")
	}
	if in.MaxMissed < 1 || in.MaxMissed > 24 {
		bad("max_missed", "1 to 24")
	}
	if len(details) > 0 {
		return uuid.Nil, model.ErrInvalidInput.WithDetails(details...)
	}
	id, err := repository.NewRepository(s.pool).CreateInstallmentOffer(ctx, gen.CreateInstallmentOfferParams{
		PlanPriceID: in.PlanPriceID, Count: int32(in.Count), TotalMinor: in.TotalMinor, //nolint:gosec // range checked above
		GraceDays: int32(in.GraceDays), MaxMissed: int32(in.MaxMissed), //nolint:gosec // range checked above
	})
	if repository.IsForeignKey(err) {
		// The composite FK also refuses recurring prices.
		return uuid.Nil, model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "plan_price_id", Message: "Installments split a one-time price",
		})
	}
	return id, catalogErr("create installment offer", err)
}

// CreateProviderPrice says how a provider sells a price, or one of its installment offers.
func (s *Service) CreateProviderPrice(ctx context.Context, in model.NewProviderPrice) (uuid.UUID, error) {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }
	if _, ok := payment.ParseProvider(in.Provider); !ok {
		bad("provider", "Unknown payment provider")
	}
	ext := in.ExternalPriceID
	if ext != nil {
		ext = nilIfEmpty(strings.TrimSpace(*ext))
	}
	switch {
	case in.InstallmentOfferID == nil && in.Collection != nil:
		bad("collection", "Only an installment offer has a collection")
	case in.InstallmentOfferID != nil:
		switch deref(in.Collection) {
		case model.CollectionProviderSchedule, model.CollectionAppLink, model.CollectionProviderFinanced:
		default:
			bad("collection", "Collection is provider_schedule, app_link or provider_financed")
		}
	}
	if len(details) > 0 {
		return uuid.Nil, model.ErrInvalidInput.WithDetails(details...)
	}
	r := repository.NewRepository(s.pool)
	if in.InstallmentOfferID != nil {
		o, err := r.GetInstallmentOffer(ctx, *in.InstallmentOfferID)
		if repository.IsNoRows(err) || (err == nil && o.PlanPriceID != in.PlanPriceID) {
			return uuid.Nil, model.ErrOfferNotFound
		}
		if err != nil {
			return uuid.Nil, fmt.Errorf("get installment offer: %w", err)
		}
		if err := s.ensureOfferProduct(ctx, r, in); err != nil {
			return uuid.Nil, err
		}
	}
	if ext == nil && in.InstallmentOfferID == nil {
		created, err := s.createOnProvider(ctx, r, in.PlanPriceID, in.Provider)
		if err != nil {
			return uuid.Nil, err
		}
		ext = &created
	}
	var id uuid.UUID
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		var err error
		if id, err = r.CreateProviderPrice(ctx, gen.CreateProviderPriceParams{
			PlanPriceID: in.PlanPriceID, InstallmentOfferID: in.InstallmentOfferID, Provider: in.Provider,
			ExternalPriceID: ext, Collection: in.Collection,
		}); err != nil {
			return err
		}
		return s.retireCoupons(ctx, r, in)
	})
	if repository.IsForeignKey(err) {
		return uuid.Nil, model.ErrPriceNotFound
	}
	if err != nil {
		return uuid.Nil, catalogErr("create provider price", err)
	}
	// The link is saved; a coupon left on the provider is logged and deleted by a later retirement.
	_ = s.deleteRetiredCoupons(ctx, gen.ListUndeletedRetiredProviderDiscountsParams{Provider: &in.Provider})
	return id, nil
}

// retireCoupons retires the provider's mirrored coupons when a recurring price
// gets a product on a provider whose coupons cover only the products they were
// made for (Creem). The next checkout with each code mirrors it again, over
// the current products, and the old mirrors are deleted on the provider.
func (s *Service) retireCoupons(ctx context.Context, r repository.Repository, in model.NewProviderPrice) error {
	g, ok := s.payments.Get(payment.Provider(in.Provider))
	if in.InstallmentOfferID != nil || !ok || !g.Capabilities().CouponsPerProduct {
		return nil
	}
	price, err := r.GetPlanPrice(ctx, in.PlanPriceID)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}
	if price.BillingType != model.BillingRecurring {
		return nil
	}
	if _, err := r.RetireProviderDiscounts(ctx, in.Provider); err != nil {
		return fmt.Errorf("retire provider discounts: %w", err)
	}
	return nil
}

// ensureOfferProduct gives an installment offer link the plain provider price
// its one-by-one payments charge through, on a provider whose payment-mode
// checkout needs a product (Creem): it creates one on the provider when the
// price has none, so staff get both links from one request.
func (s *Service) ensureOfferProduct(ctx context.Context, r repository.Repository, in model.NewProviderPrice) error {
	if !s.needsProduct(model.OfferProvider{Provider: payment.Provider(in.Provider), Collection: deref(in.Collection)}) {
		return nil
	}
	if _, err := r.GetProviderPrice(ctx, gen.GetProviderPriceParams{
		PlanPriceID: in.PlanPriceID, InstallmentOfferID: in.InstallmentOfferID, Provider: in.Provider,
	}); err == nil {
		return model.ErrCatalogDuplicate // checked first, so a refused link leaves no product behind
	} else if !repository.IsNoRows(err) {
		return fmt.Errorf("get provider price: %w", err)
	}
	pp, err := r.GetProviderPrice(ctx, gen.GetProviderPriceParams{PlanPriceID: in.PlanPriceID, Provider: in.Provider})
	switch {
	case err == nil && deref(pp.ExternalPriceID) != "":
		return nil
	case err == nil:
		// A plain link with no product: staff must replace it, as rows are never edited.
		return model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "provider", Message: "The price's " + in.Provider + " link has no product. Deactivate it and add it again first.",
		})
	case !repository.IsNoRows(err):
		return fmt.Errorf("get provider price: %w", err)
	}
	created, err := s.createOnProvider(ctx, r, in.PlanPriceID, in.Provider)
	if err != nil {
		return err
	}
	_, err = r.CreateProviderPrice(ctx, gen.CreateProviderPriceParams{
		PlanPriceID: in.PlanPriceID, Provider: in.Provider, ExternalPriceID: &created,
	})
	return catalogErr("create provider price", err)
}

// createOnProvider has the provider create the product or price a plain plan
// price sells through, and returns its ID. It refuses first when an active link
// already exists, so a refused link never leaves an orphan on the provider.
func (s *Service) createOnProvider(ctx context.Context, r repository.Repository, planPriceID uuid.UUID, provider string) (string, error) {
	row, err := r.GetPlanPrice(ctx, planPriceID)
	if repository.IsNoRows(err) {
		return "", model.ErrPriceNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get price: %w", err)
	}
	if !row.IsActive {
		return "", model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "plan_price_id", Message: "This price is no longer on sale"})
	}
	_, err = r.GetProviderPrice(ctx, gen.GetProviderPriceParams{PlanPriceID: planPriceID, Provider: provider})
	switch {
	case err == nil:
		return "", model.ErrCatalogDuplicate
	case !repository.IsNoRows(err):
		return "", fmt.Errorf("get provider price: %w", err)
	}
	g, err := s.gateway(payment.Provider(provider))
	if err != nil {
		return "", model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "provider", Message: provider + " is not enabled on this server",
		})
	}
	req := payment.PriceRequest{
		PlanPriceID: planPriceID, Name: "Relay " + row.PlanName + " (lifetime)", AmountMinor: row.AmountMinor, Currency: row.Currency,
	}
	if row.BillingType == model.BillingRecurring {
		req.Interval = deref(row.Interval)
		req.Name = "Relay " + row.PlanName + " (monthly)"
		if req.Interval == model.IntervalYear {
			req.Name = "Relay " + row.PlanName + " (yearly)"
		}
	}
	external, err := g.CreatePrice(ctx, req)
	if errors.Is(err, payment.ErrInvalidRequest) {
		s.logger().Warn("payment provider refused a price", "provider", provider, "err", err)
		return "", model.ErrInvalidInput.WithDetails(apperr.Detail{
			Field: "provider", Message: provider + " refused this price",
		})
	}
	if err != nil {
		return "", s.gatewayErr("create provider price", err)
	}
	return external, nil
}

// Catalog rows an admin can deactivate.
const (
	CatalogPlan          = "plan"
	CatalogPrice         = "price"
	CatalogOffer         = "installment_offer"
	CatalogProviderPrice = "provider_price"
)

// DeactivateCatalogRow takes a plan, price, installment offer or provider
// price off sale. What was already bought keeps working.
func (s *Service) DeactivateCatalogRow(ctx context.Context, kind string, id uuid.UUID) error {
	r := repository.NewRepository(s.pool)
	var (
		n   int64
		err error
	)
	switch kind {
	case CatalogPlan:
		n, err = r.DeactivatePlan(ctx, id)
	case CatalogPrice:
		n, err = r.DeactivatePlanPrice(ctx, id)
	case CatalogOffer:
		n, err = r.DeactivateInstallmentOffer(ctx, id)
	case CatalogProviderPrice:
		n, err = r.DeactivateProviderPrice(ctx, id)
	default:
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "kind", Message: "Unknown catalog row"})
	}
	if err != nil {
		return fmt.Errorf("deactivate %s: %w", kind, err)
	}
	if n == 0 {
		return model.ErrCatalogRowNotFound
	}
	return nil
}

// catalogErr maps constraint errors from a catalog insert.
func catalogErr(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case repository.IsDuplicate(err):
		return model.ErrCatalogDuplicate
	case repository.IsCheck(err):
		return model.ErrInvalidInput.Wrap(err)
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}

// AdminWorkspaces lists workspaces, newest first, with the plan each is on.
func (s *Service) AdminWorkspaces(ctx context.Context, search string) ([]model.AdminWorkspace, error) {
	r := repository.NewRepository(s.pool)
	rows, err := r.AdminListWorkspaces(ctx, strings.TrimSpace(search))
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	free, err := freeLimits(ctx, r)
	if err != nil {
		return nil, err
	}
	out := make([]model.AdminWorkspace, 0, len(rows))
	for _, w := range rows {
		ws := model.AdminWorkspace{
			ID: w.ID, Name: w.Name, OwnerEmail: w.OwnerEmail, Plan: w.Plan, TrialEndsAt: w.TrialEndsAt, CreatedAt: w.CreatedAt,
			Limits: free,
		}
		if w.GrantID != nil {
			planLimits, err := model.ParseLimits(w.GrantPlanLimits)
			if err != nil {
				return nil, fmt.Errorf("grant %s plan: %w", *w.GrantID, err)
			}
			overrides, err := model.ParseLimits(w.GrantOverrides)
			if err != nil {
				return nil, fmt.Errorf("grant %s overrides: %w", *w.GrantID, err)
			}
			ws.Grant = &model.AdminGrant{
				ID: *w.GrantID, PlanCode: deref(w.GrantPlanCode), PlanName: deref(w.GrantPlanName),
				Source: deref(w.GrantSource), Status: deref(w.GrantStatus), EndsAt: w.GrantEndsAt, Overrides: overrides,
			}
			if ws.Grant.Status == model.GrantActive {
				ws.Limits = planLimits.With(overrides)
			}
		}
		out = append(out, ws)
	}
	return out, nil
}

// GrantPlanManually puts a workspace on a plan by hand: a comp, a partner deal
// or a support fix. A plan paid through a subscription or lifetime deal is
// left alone; billing would put it back on the next payment.
func (s *Service) GrantPlanManually(ctx context.Context, in model.ManualGrantInput) error {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "reason", Message: "Say why, for the record"})
	}
	if in.EndsAt != nil && !in.EndsAt.After(s.now()) {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "ends_at", Message: "The end must be in the future"})
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if _, err := s.adminGrant(ctx, r, in.WorkspaceID); err != nil {
			return err
		}
		createdBy := in.CreatedBy
		_, err := s.ActivateGrant(ctx, tx, in.WorkspaceID, model.GrantInput{
			PlanID: in.PlanID, Source: model.GrantManual, EndsAt: in.EndsAt, CreatedBy: &createdBy, Reason: &reason,
		})
		return err
	})
}

// EndGrantManually ends a manual grant or a trial: the workspace goes to the free plan.
func (s *Service) EndGrantManually(ctx context.Context, workspaceID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		g, err := s.adminGrant(ctx, repository.NewRepository(tx), workspaceID)
		if err != nil {
			return err
		}
		if g == nil {
			return model.ErrNoGrant
		}
		return s.EndGrant(ctx, tx, workspaceID, g.ID)
	})
}

// RestoreGrantManually reactivates a suspended grant (a missed installment, a
// lost dispute) once support has sorted it out.
func (s *Service) RestoreGrantManually(ctx context.Context, workspaceID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if ok, err := r.AdminWorkspaceExists(ctx, workspaceID); err != nil {
			return fmt.Errorf("workspace exists: %w", err)
		} else if !ok {
			return model.ErrWorkspaceNotFound
		}
		g, err := r.LockCurrentGrant(ctx, workspaceID)
		if repository.IsNoRows(err) {
			return model.ErrNoGrant
		}
		if err != nil {
			return fmt.Errorf("lock current grant: %w", err)
		}
		if g.Status != model.GrantSuspended {
			return model.ErrGrantNotSuspended
		}
		return s.RestoreGrant(ctx, tx, workspaceID, g.ID)
	})
}

// SetGrantOverrides replaces a workspace's own limits on top of its plan's.
// They live on the current grant, so a plan change starts without them.
func (s *Service) SetGrantOverrides(ctx context.Context, in model.GrantOverridesEdit) error {
	reason := strings.TrimSpace(in.Reason)
	details := in.Overrides.CheckOverrides("overrides")
	if reason == "" {
		details = append(details, apperr.Detail{Field: "reason", Message: "Say why, for the record"})
	}
	if len(details) > 0 {
		return model.ErrInvalidInput.WithDetails(details...)
	}
	after, err := json.Marshal(in.Overrides)
	if err != nil {
		return fmt.Errorf("encode overrides: %w", err)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if ok, err := r.AdminWorkspaceExists(ctx, in.WorkspaceID); err != nil {
			return fmt.Errorf("workspace exists: %w", err)
		} else if !ok {
			return model.ErrWorkspaceNotFound
		}
		g, err := r.LockCurrentGrant(ctx, in.WorkspaceID)
		if repository.IsNoRows(err) {
			return model.ErrNoGrant
		}
		if err != nil {
			return fmt.Errorf("lock current grant: %w", err)
		}
		if _, err := r.SetGrantOverrides(ctx, gen.SetGrantOverridesParams{
			Overrides: after, WorkspaceID: in.WorkspaceID, GrantID: g.ID,
		}); err != nil {
			return fmt.Errorf("set grant overrides: %w", err)
		}
		return r.InsertLimitChange(ctx, gen.InsertLimitChangeParams{
			GrantID: &g.ID, ChangedBy: &in.ChangedBy, Before: g.Overrides, After: after, Reason: reason,
		})
	})
}

// adminGrant locks the workspace's current grant for a manual change and
// refuses one that payments own. It returns nil on the free plan.
func (s *Service) adminGrant(ctx context.Context, r repository.Repository, workspaceID uuid.UUID) (*gen.LockCurrentGrantRow, error) {
	ok, err := r.AdminWorkspaceExists(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace exists: %w", err)
	}
	if !ok {
		return nil, model.ErrWorkspaceNotFound
	}
	g, err := r.LockCurrentGrant(ctx, workspaceID)
	if repository.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock current grant: %w", err)
	}
	if g.Source == model.GrantFromSubscription || g.Source == model.GrantFromOrder {
		return nil, model.ErrGrantFromPayment
	}
	return &g, nil
}

// DisputedInvoices lists invoices under an open dispute, oldest first. Creem
// sends no event when a dispute closes, so staff close those here.
func (s *Service) DisputedInvoices(ctx context.Context) ([]model.DisputedInvoice, error) {
	rows, err := repository.NewRepository(s.pool).AdminListDisputedInvoices(ctx)
	if err != nil {
		return nil, fmt.Errorf("list disputed invoices: %w", err)
	}
	out := make([]model.DisputedInvoice, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.DisputedInvoice{
			ID: r.ID, WorkspaceID: r.WorkspaceID, WorkspaceName: r.WorkspaceName, Number: deref(r.Number),
			Provider: r.Provider, TotalMinor: r.TotalMinor, Currency: r.Currency, PaidAt: r.PaidAt, DisputedSince: r.UpdatedAt,
		})
	}
	return out, nil
}

// CloseDispute records the outcome of a dispute the provider reports no
// outcome for, as the provider's own dispute-closed event would: won puts the
// invoice back to paid, lost suspends the plan it paid for. The decision is
// stored as a payment event, so who closed it and why stay on record.
func (s *Service) CloseDispute(ctx context.Context, in model.DisputeClose) error {
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "reason", Message: "Say how the dispute was settled"})
	}
	outcome := "lost"
	if in.Won {
		outcome = "won"
	}
	payload, err := json.Marshal(map[string]string{"outcome": outcome, "reason": reason, "closed_by": in.ClosedBy.String()})
	if err != nil {
		return fmt.Errorf("encode dispute decision: %w", err)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		inv, err := r.AdminGetInvoice(ctx, in.InvoiceID)
		if repository.IsNoRows(err) {
			return model.ErrInvoiceNotFound
		}
		if err != nil {
			return fmt.Errorf("get invoice: %w", err)
		}
		if inv.Status != model.InvoiceDisputed || inv.ExternalPaymentID == nil {
			return model.ErrNotDisputed
		}
		ev := payment.Event{
			Provider: payment.Provider(inv.Provider), ExternalID: "staff_dispute_closed_" + uuid.NewString(),
			Type: payment.EventDisputeClosed, ProviderType: "staff.dispute_closed",
			Payment: &payment.Payment{ExternalID: *inv.ExternalPaymentID, Status: outcome},
		}
		id, err := r.InsertPaymentEvent(ctx, gen.InsertPaymentEventParams{
			Provider: inv.Provider, ExternalEventID: ev.ExternalID, EventType: string(ev.Type), Payload: payload,
		})
		if err != nil {
			return fmt.Errorf("insert payment event: %w", err)
		}
		if _, err := r.SetPaymentEventWorkspace(ctx, gen.SetPaymentEventWorkspaceParams{
			WorkspaceID: &inv.WorkspaceID, PaymentEventID: id,
		}); err != nil {
			return fmt.Errorf("set payment event workspace: %w", err)
		}
		if err := s.onDisputeClosed(ctx, tx, inv.WorkspaceID, ev); err != nil {
			return err
		}
		s.log.InfoContext(ctx, "billing: dispute closed by staff", "invoice_id", inv.ID, "workspace_id", inv.WorkspaceID,
			"outcome", outcome, "closed_by", in.ClosedBy)
		return markProcessed(ctx, r, id, s.now())
	})
}
