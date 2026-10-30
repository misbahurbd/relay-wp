package service

import (
	"context"
	"encoding/json"
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
		version, err := r.NextPlanVersion(ctx, code)
		if err != nil {
			return fmt.Errorf("next plan version: %w", err)
		}
		id, err = r.CreatePlan(ctx, gen.CreatePlanParams{
			Code: code, Version: version, Name: d.Name, Description: d.Description, Features: d.Features,
			Recommended: d.Recommended, TrialDays: int32(d.TrialDays), //nolint:gosec // range checked above
			TrialRequiresCard: d.TrialRequiresCard, Limits: limits, SortOrder: int32(d.SortOrder), //nolint:gosec // a small display order
		})
		return err
	})
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
	}
	id, err := r.CreateProviderPrice(ctx, gen.CreateProviderPriceParams{
		PlanPriceID: in.PlanPriceID, InstallmentOfferID: in.InstallmentOfferID, Provider: in.Provider,
		ExternalPriceID: ext, Collection: in.Collection,
	})
	if repository.IsForeignKey(err) {
		return uuid.Nil, model.ErrPriceNotFound
	}
	return id, catalogErr("create provider price", err)
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
		if _, err := r.SetGrantOverrides(ctx, gen.SetGrantOverridesParams{GrantID: g.ID, Overrides: after}); err != nil {
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
