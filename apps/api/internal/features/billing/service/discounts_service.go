package service

import (
	"context"
	"fmt"
	"math"
	"regexp"
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

// Track T3: discount codes (Q18).

// discountCode is the shape discounts.code allows, after upper-casing.
var discountCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{2,39}$`)

// PreviewDiscount checks a code against a price before checkout and returns
// the reduced price (and reduced installments for an offer), or the model
// error that says why the code doesn't apply.
func (s *Service) PreviewDiscount(ctx context.Context, workspaceID uuid.UUID, in model.DiscountPreviewInput) (model.DiscountPreview, error) {
	price, err := s.loadPrice(ctx, repository.NewRepository(s.pool), in.PlanPriceID)
	if err != nil {
		return model.DiscountPreview{}, err
	}
	if !price.IsActive {
		return model.DiscountPreview{}, model.ErrPriceNotFound
	}
	var offer *model.InstallmentOffer
	if in.InstallmentOfferID != nil {
		o, err := s.InstallmentOffer(ctx, *in.InstallmentOfferID)
		if err != nil {
			return model.DiscountPreview{}, err
		}
		if o.PlanPriceID != price.ID {
			return model.DiscountPreview{}, model.ErrOfferNotFound
		}
		offer = &o
	}
	d, err := s.ApplyDiscount(ctx, workspaceID, in.Code, price.Price, offer)
	if err != nil {
		return model.DiscountPreview{}, err
	}
	subtotal := price.AmountMinor
	if offer != nil {
		subtotal = offer.TotalMinor
	}
	out := model.DiscountPreview{
		Code: d.Discount.Code, SubtotalMinor: subtotal, DiscountMinor: d.DiscountMinor,
		TotalMinor: subtotal - d.DiscountMinor, Currency: price.Currency, Duration: d.Discount.Duration,
		DurationMonths: d.Discount.DurationMonths,
	}
	if price.BillingType == model.BillingOneTime {
		// One-time payments always apply once.
		out.Duration, out.DurationMonths = model.DurationOnce, nil
	}
	if offer != nil {
		out.Installments = model.SplitInstallments(out.TotalMinor, offer.Count)
	}
	return out, nil
}

// ApplyDiscount validates code for this workspace and price (active, in its
// dates, plan and price type, installments allowed, per-workspace limit) and
// works out the amount off: of the price, or of offer's total when offer is
// set. It reserves nothing. Track T4's QuoteOrder uses it.
func (s *Service) ApplyDiscount(ctx context.Context, workspaceID uuid.UUID, code string, price model.Price, offer *model.InstallmentOffer) (model.AppliedDiscount, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return model.AppliedDiscount{}, model.ErrDiscountNotFound
	}
	r := repository.NewRepository(s.pool)
	row, err := r.GetDiscountByCode(ctx, code)
	if repository.IsNoRows(err) {
		return model.AppliedDiscount{}, model.ErrDiscountNotFound
	}
	if err != nil {
		return model.AppliedDiscount{}, fmt.Errorf("get discount: %w", err)
	}
	d := toDiscount(gen.GetDiscountRow(row))
	now := s.now()
	switch {
	case !d.IsActive:
		return model.AppliedDiscount{}, model.ErrDiscountInactive
	case now.Before(d.StartsAt):
		return model.AppliedDiscount{}, model.ErrDiscountNotStarted
	case d.ExpiresAt != nil && !now.Before(*d.ExpiresAt):
		return model.AppliedDiscount{}, model.ErrDiscountExpired
	case d.MaxRedemptions != nil && d.RedeemedCount >= *d.MaxRedemptions:
		return model.AppliedDiscount{}, model.ErrDiscountUsedUp
	case d.PlanID != nil && *d.PlanID != price.PlanID:
		return model.AppliedDiscount{}, model.ErrDiscountWrongPlan
	case d.AppliesTo == model.AppliesToRecurring && price.BillingType != model.BillingRecurring,
		d.AppliesTo == model.AppliesToOneTime && price.BillingType != model.BillingOneTime:
		return model.AppliedDiscount{}, model.ErrDiscountWrongPrice
	case offer != nil && !d.AllowInstallments:
		return model.AppliedDiscount{}, model.ErrDiscountNoInstallments
	case d.Kind == model.DiscountAmount && deref(d.Currency) != price.Currency:
		return model.AppliedDiscount{}, model.ErrDiscountCurrency
	}
	used, err := r.CountWorkspaceRedemptions(ctx, gen.CountWorkspaceRedemptionsParams{
		DiscountID: d.ID, WorkspaceID: workspaceID,
	})
	if err != nil {
		return model.AppliedDiscount{}, fmt.Errorf("count redemptions: %w", err)
	}
	if int(used) >= d.PerWorkspaceLimit {
		return model.AppliedDiscount{}, model.ErrDiscountWorkspaceLimit
	}

	base := price.AmountMinor
	if offer != nil {
		base = offer.TotalMinor
	}
	out := model.AppliedDiscount{Discount: d, DiscountMinor: amountOff(d, base)}
	if price.BillingType == model.BillingRecurring && d.Duration == model.DurationRepeating && d.DurationMonths != nil {
		end := now.AddDate(0, *d.DurationMonths, 0)
		out.EndsAt = &end
	}
	return out, nil
}

// amountOff is what d takes off base, never more than base.
func amountOff(d model.Discount, base int64) int64 {
	var off int64
	switch d.Kind {
	case model.DiscountPercent:
		off = base * int64(deref(d.PercentBps)) / 10000
	case model.DiscountAmount:
		off = deref(d.AmountOffMinor)
	}
	return min(max(off, 0), base)
}

// ReserveDiscount holds one use of d for a checkout session in tx, through
// the repository's single-statement ReserveDiscount so max_redemptions holds
// under concurrency. model.ErrDiscountUsedUp when nothing is left.
func (s *Service) ReserveDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, d model.AppliedDiscount) error {
	_, err := repository.NewRepository(tx).ReserveDiscount(ctx, gen.ReserveDiscountParams{
		WorkspaceID: workspaceID, CheckoutSessionID: sessionID, AmountMinor: d.DiscountMinor,
		DiscountID: d.Discount.ID, Now: s.now(),
	})
	if repository.IsNoRows(err) {
		return model.ErrDiscountUsedUp
	}
	if err != nil {
		return fmt.Errorf("reserve discount: %w", err)
	}
	return nil
}

// RedeemDiscount turns a session's reservation into a redemption in tx, once it is paid.
func (s *Service) RedeemDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID, subscriptionID, orderID *uuid.UUID) error {
	now := s.now()
	if _, err := repository.NewRepository(tx).RedeemDiscount(ctx, gen.RedeemDiscountParams{
		Now: &now, SubscriptionID: subscriptionID, OrderID: orderID, WorkspaceID: workspaceID, CheckoutSessionID: sessionID,
	}); err != nil {
		return fmt.Errorf("redeem discount: %w", err)
	}
	return nil
}

// ReleaseDiscount frees a failed or expired session's reservation in tx and
// gives the use back. A session with none is a no-op.
func (s *Service) ReleaseDiscount(ctx context.Context, tx pgx.Tx, workspaceID, sessionID uuid.UUID) error {
	if _, err := repository.NewRepository(tx).ReleaseDiscount(ctx, gen.ReleaseDiscountParams{
		WorkspaceID: workspaceID, CheckoutSessionID: sessionID,
	}); err != nil {
		return fmt.Errorf("release discount: %w", err)
	}
	return nil
}

// providerCoupon returns the provider coupon mirroring d, creating it through
// the gateway the first time a subscription the provider renews uses d.
func (s *Service) providerCoupon(ctx context.Context, g payment.Gateway, d model.Discount) (string, error) {
	r := repository.NewRepository(s.pool)
	params := gen.GetProviderDiscountParams{DiscountID: d.ID, Provider: string(g.Provider())}
	pd, err := r.GetProviderDiscount(ctx, params)
	if err == nil {
		return pd.ExternalCouponID, nil
	}
	if !repository.IsNoRows(err) {
		return "", fmt.Errorf("get provider discount: %w", err)
	}
	req := payment.CouponRequest{
		DiscountID: d.ID, Code: d.Code, Name: d.Name, Currency: deref(d.Currency), Duration: d.Duration,
		DurationMonths: deref(d.DurationMonths),
	}
	if d.Kind == model.DiscountPercent {
		req.PercentBps = deref(d.PercentBps)
	} else {
		req.AmountOffMinor = deref(d.AmountOffMinor)
	}
	c, err := g.CreateCoupon(ctx, req)
	if err != nil {
		return "", s.gatewayErr("create coupon", err)
	}
	created, err := r.CreateProviderDiscount(ctx, gen.CreateProviderDiscountParams{
		DiscountID: d.ID, Provider: string(g.Provider()), ExternalCouponID: c.ExternalCouponID,
		ExternalPromotionCodeID: nilIfEmpty(c.ExternalPromotionCodeID),
	})
	if err != nil {
		// Another checkout mirrored it first: use theirs.
		if pd, again := r.GetProviderDiscount(ctx, params); again == nil {
			return pd.ExternalCouponID, nil
		}
		return "", fmt.Errorf("save provider discount: %w", err)
	}
	return created.ExternalCouponID, nil
}

// CreateDiscount adds a discount code. For the billing discounts CLI command.
func (s *Service) CreateDiscount(ctx context.Context, in model.NewDiscount) (model.Discount, error) {
	params, err := s.discountParams(in)
	if err != nil {
		return model.Discount{}, err
	}
	r := repository.NewRepository(s.pool)
	id, err := r.CreateDiscount(ctx, params)
	switch {
	case repository.IsUnique(err, repository.DiscountCodeKey):
		return model.Discount{}, model.ErrDiscountCodeTaken
	case repository.IsForeignKey(err):
		return model.Discount{}, model.ErrPlanNotFound
	case err != nil:
		return model.Discount{}, fmt.Errorf("create discount: %w", err)
	}
	row, err := r.GetDiscount(ctx, id)
	if err != nil {
		return model.Discount{}, fmt.Errorf("get discount: %w", err)
	}
	return toDiscount(row), nil
}

// discountParams checks a new discount against the rules the table enforces,
// so the CLI gets a clear message instead of a constraint name.
func (s *Service) discountParams(in model.NewDiscount) (gen.CreateDiscountParams, error) {
	var details []apperr.Detail
	bad := func(field, msg string) { details = append(details, apperr.Detail{Field: field, Message: msg}) }

	code := strings.ToUpper(strings.TrimSpace(in.Code))
	if !discountCode.MatchString(code) {
		bad("code", "Use 3 to 40 letters, digits, - or _")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = code
	}
	p := gen.CreateDiscountParams{
		Code: code, Name: name, Kind: in.Kind, Duration: in.Duration, AppliesTo: in.AppliesTo, PlanID: in.PlanID,
		AllowInstallments: in.AllowInstallments, StartsAt: in.StartsAt, ExpiresAt: in.ExpiresAt, CreatedBy: in.CreatedBy,
	}
	switch in.Kind {
	case model.DiscountPercent:
		if in.PercentBps == nil || *in.PercentBps < 1 || *in.PercentBps > 10000 {
			bad("percent_bps", "A percent discount needs 1 to 10000 basis points")
		} else {
			p.PercentBps = ptrTo(int32(*in.PercentBps))
		}
		if in.AmountOffMinor != nil {
			bad("amount_off_minor", "A percent discount takes no amount")
		}
	case model.DiscountAmount:
		if in.AmountOffMinor == nil || *in.AmountOffMinor <= 0 {
			bad("amount_off_minor", "An amount discount needs a positive amount")
		}
		if in.Currency == nil || !currencyCode.MatchString(strings.ToUpper(*in.Currency)) {
			bad("currency", "An amount discount needs a three-letter currency")
		} else {
			p.Currency = ptrTo(strings.ToUpper(*in.Currency))
		}
		if in.PercentBps != nil {
			bad("percent_bps", "An amount discount takes no percent")
		}
		p.AmountOffMinor = in.AmountOffMinor
	default:
		bad("kind", "Kind is percent or amount")
	}
	if p.Duration == "" {
		p.Duration = model.DurationOnce
	}
	switch p.Duration {
	case model.DurationRepeating:
		if in.DurationMonths == nil || *in.DurationMonths < 1 || *in.DurationMonths > 36 {
			bad("duration_months", "A repeating discount lasts 1 to 36 months")
		} else {
			p.DurationMonths = ptrTo(int32(*in.DurationMonths))
		}
	case model.DurationOnce, model.DurationForever:
		if in.DurationMonths != nil {
			bad("duration_months", "Only a repeating discount takes months")
		}
	default:
		bad("duration", "Duration is once, repeating or forever")
	}
	if p.AppliesTo == "" {
		p.AppliesTo = model.AppliesToAll
	}
	switch p.AppliesTo {
	case model.AppliesToAll, model.AppliesToRecurring, model.AppliesToOneTime:
	default:
		bad("applies_to", "Applies to all, recurring or one_time")
	}
	if in.MaxRedemptions != nil {
		if *in.MaxRedemptions < 1 || *in.MaxRedemptions > math.MaxInt32 {
			bad("max_redemptions", "The cap must be at least 1")
		} else {
			p.MaxRedemptions = ptrTo(int32(*in.MaxRedemptions)) //nolint:gosec // range checked above
		}
	}
	switch {
	case in.PerWorkspaceLimit == 0:
		p.PerWorkspaceLimit = 1
	case in.PerWorkspaceLimit < 0 || in.PerWorkspaceLimit > math.MaxInt32:
		bad("per_workspace_limit", "The per-workspace limit must be at least 1")
	default:
		p.PerWorkspaceLimit = int32(in.PerWorkspaceLimit) //nolint:gosec // range checked above
	}
	if p.StartsAt.IsZero() {
		p.StartsAt = s.now()
	}
	if p.ExpiresAt != nil && !p.ExpiresAt.After(p.StartsAt) {
		bad("expires_at", "The code must expire after it starts")
	}
	if len(details) > 0 {
		return gen.CreateDiscountParams{}, model.ErrInvalidInput.WithDetails(details...)
	}
	return p, nil
}

// ListDiscounts lists discount codes, newest first. For the CLI.
func (s *Service) ListDiscounts(ctx context.Context, activeOnly bool) ([]model.Discount, error) {
	rows, err := repository.NewRepository(s.pool).ListDiscounts(ctx, activeOnly)
	if err != nil {
		return nil, fmt.Errorf("list discounts: %w", err)
	}
	out := make([]model.Discount, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDiscount(gen.GetDiscountRow(r)))
	}
	return out, nil
}

// DeactivateDiscount stops a code from being used again, and its provider
// coupons with it. Redemptions already made stand. For the CLI.
func (s *Service) DeactivateDiscount(ctx context.Context, code string) error {
	row, err := repository.NewRepository(s.pool).GetDiscountByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if repository.IsNoRows(err) {
		return model.ErrDiscountNotFound
	}
	if err != nil {
		return fmt.Errorf("get discount: %w", err)
	}
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		r := repository.NewRepository(tx)
		if _, err := r.DeactivateDiscount(ctx, row.ID); err != nil {
			return fmt.Errorf("deactivate discount: %w", err)
		}
		if _, err := r.DeactivateProviderDiscounts(ctx, row.ID); err != nil {
			return fmt.Errorf("deactivate provider discounts: %w", err)
		}
		return nil
	})
}

// discountEndsAt is when a discount stops applying to a subscription that
// started its first period at start: after its months for repeating, after
// the first period for once, never for forever.
func discountEndsAt(d model.Discount, start time.Time, firstPeriodEnd *time.Time) *time.Time {
	switch d.Duration {
	case model.DurationRepeating:
		end := start.AddDate(0, deref(d.DurationMonths), 0)
		return &end
	case model.DurationOnce:
		return firstPeriodEnd
	default:
		return nil
	}
}

func toDiscount(r gen.GetDiscountRow) model.Discount {
	d := model.Discount{
		ID: r.ID, Code: r.Code, Name: r.Name, Kind: r.Kind, AmountOffMinor: r.AmountOffMinor, Currency: r.Currency,
		Duration: r.Duration, AppliesTo: r.AppliesTo, PlanID: r.PlanID, AllowInstallments: r.AllowInstallments,
		PerWorkspaceLimit: int(r.PerWorkspaceLimit), RedeemedCount: int(r.RedeemedCount), StartsAt: r.StartsAt,
		ExpiresAt: r.ExpiresAt, IsActive: r.IsActive,
	}
	if r.PercentBps != nil {
		d.PercentBps = ptrTo(int(*r.PercentBps))
	}
	if r.DurationMonths != nil {
		d.DurationMonths = ptrTo(int(*r.DurationMonths))
	}
	if r.MaxRedemptions != nil {
		d.MaxRedemptions = ptrTo(int(*r.MaxRedemptions))
	}
	return d
}

// currencyCode is ISO 4217, upper case.
var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

// ptrTo returns a pointer to v.
func ptrTo[T any](v T) *T { return &v }

// nilIfEmpty returns nil for "", else a pointer to s.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
