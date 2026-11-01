package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
)

// Consume uses n of monthly resource key in the current month, unless that
// would pass the plan's cap. ok is false at the cap; the feature refuses with
// its own error. It runs outside the feature's transaction, so a feature whose
// action then fails calls Release with the returned period (ADR 0008).
func (s *Service) Consume(ctx context.Context, workspaceID uuid.UUID, key string, n int) (period time.Time, ok bool, err error) {
	if err := monthly(key, n); err != nil {
		return time.Time{}, false, err
	}
	limit, err := s.Limit(ctx, workspaceID, key)
	if err != nil {
		return time.Time{}, false, err
	}
	period = model.PeriodStart(s.now())
	if limit != model.Unlimited && n > limit {
		return period, false, nil
	}
	_, err = repository.NewRepository(s.pool).ConsumeUsage(ctx, gen.ConsumeUsageParams{
		WorkspaceID: workspaceID, Resource: key, PeriodStart: date(period),
		N: int32(n), Cap: int32(min(limit, math.MaxInt32)), //nolint:gosec // n checked by monthly, limit capped
	})
	if repository.IsNoRows(err) {
		return period, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("consume %s: %w", key, err)
	}
	return period, true, nil
}

// Release gives back n of monthly resource key consumed in the month that
// starts at period, when the action it was consumed for didn't happen.
func (s *Service) Release(ctx context.Context, workspaceID uuid.UUID, key string, period time.Time, n int) error {
	if err := monthly(key, n); err != nil {
		return err
	}
	_, err := repository.NewRepository(s.pool).ReleaseUsage(ctx, gen.ReleaseUsageParams{
		WorkspaceID: workspaceID, Resource: key, PeriodStart: date(model.PeriodStart(period)),
		N: int32(n), //nolint:gosec // checked by monthly
	})
	if err != nil {
		return fmt.Errorf("release %s: %w", key, err)
	}
	return nil
}

// Usage is how much of each monthly resource the workspace used this month,
// against its current plan's caps, in catalog order.
func (s *Service) Usage(ctx context.Context, workspaceID uuid.UUID) (model.Usage, error) {
	p, err := s.CurrentPlan(ctx, workspaceID)
	if err != nil {
		return model.Usage{}, err
	}
	start := model.PeriodStart(s.now())
	rows, err := repository.NewRepository(s.pool).ListUsage(ctx, gen.ListUsageParams{WorkspaceID: workspaceID, PeriodStart: date(start)})
	if err != nil {
		return model.Usage{}, fmt.Errorf("list usage: %w", err)
	}
	used := make(map[string]int, len(rows))
	for _, r := range rows {
		used[r.Resource] = int(r.Used)
	}
	u := model.Usage{PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0)}
	for _, r := range model.Resources {
		if r.Kind == model.KindMonthly {
			u.Resources = append(u.Resources, model.MonthlyUsage{Resource: r.Key, Used: used[r.Key], Limit: p.Limits.Of(r.Key)})
		}
	}
	return u, nil
}

// monthly refuses a key that isn't a monthly resource, or an n that isn't a
// positive int32: a programming error, not a user's.
func monthly(key string, n int) error {
	r, ok := model.ResourceByKey(key)
	if !ok || r.Kind != model.KindMonthly {
		return fmt.Errorf("usage: %q is not a monthly resource", key)
	}
	if n < 1 || n > math.MaxInt32 {
		return fmt.Errorf("usage: %d %s is out of range", n, key)
	}
	return nil
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }
