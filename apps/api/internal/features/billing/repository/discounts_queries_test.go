package repository_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
)

// seedCheckouts makes a workspace with a plan price and n pending checkout sessions.
func seedCheckouts(t *testing.T, pool *pgxpool.Pool, n int) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	ws := uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name) VALUES ($1, 'Billing test')`, ws); err != nil {
		t.Fatal(err)
	}
	r := repository.NewRepository(pool)
	planID, err := r.CreatePlan(ctx, gen.CreatePlanParams{
		Code: "test_" + uuid.NewString()[:8], Version: 1, Name: "Test", Limits: []byte(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	priceID, err := r.CreatePlanPrice(ctx, gen.CreatePlanPriceParams{
		PlanID: planID, BillingType: "one_time", Currency: "USD", AmountMinor: 30000,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := make([]uuid.UUID, n)
	for i := range sessions {
		s, err := r.CreateCheckoutSession(ctx, gen.CreateCheckoutSessionParams{
			WorkspaceID: ws, PlanPriceID: priceID, Purpose: "ltd", Provider: "stripe", AmountMinor: 30000,
			Currency: "USD", IdempotencyKey: uuid.NewString(), ExpiresAt: time.Now().Add(time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		sessions[i] = s.ID
	}
	return ws, sessions
}

func TestReserveDiscountHoldsTheCapUnderConcurrency(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	r := repository.NewRepository(pool)
	ws, sessions := seedCheckouts(t, pool, 20)

	maxRedemptions, perWorkspace := int32(5), int32(100)
	discountID, err := r.CreateDiscount(ctx, gen.CreateDiscountParams{
		Code: "cap" + uuid.NewString()[:6], Name: "Cap test", Kind: "amount", AmountOffMinor: ptr(int64(6000)),
		Currency: ptr("USD"), Duration: "once", AppliesTo: "all", AllowInstallments: true,
		MaxRedemptions: &maxRedemptions, PerWorkspaceLimit: perWorkspace, StartsAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}

	var won atomic.Int32
	var wg sync.WaitGroup
	for _, s := range sessions {
		wg.Go(func() {
			_, err := r.ReserveDiscount(ctx, gen.ReserveDiscountParams{
				WorkspaceID: ws, CheckoutSessionID: s, AmountMinor: 6000, DiscountID: discountID, Now: time.Now(),
			})
			switch {
			case err == nil:
				won.Add(1)
			case !repository.IsNoRows(err):
				t.Errorf("reserve: %v", err)
			}
		})
	}
	wg.Wait()
	if won.Load() != maxRedemptions {
		t.Fatalf("%d reservations went through, want exactly %d", won.Load(), maxRedemptions)
	}

	// Releasing one gives the use back, in the same statement.
	var reserved uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT checkout_session_id FROM discount_redemptions
		WHERE discount_id = $1 AND status = 'reserved' LIMIT 1`, discountID).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if n, err := r.ReleaseDiscount(ctx, gen.ReleaseDiscountParams{WorkspaceID: ws, CheckoutSessionID: reserved}); err != nil || n != 1 {
		t.Fatalf("release: %d rows, %v", n, err)
	}
	if n, err := r.ReleaseDiscount(ctx, gen.ReleaseDiscountParams{WorkspaceID: ws, CheckoutSessionID: reserved}); err != nil || n != 0 {
		t.Fatalf("second release should change nothing: %d rows, %v", n, err)
	}
	d, err := r.GetDiscount(ctx, discountID)
	if err != nil {
		t.Fatal(err)
	}
	if d.RedeemedCount != maxRedemptions-1 {
		t.Errorf("redeemed_count = %d, want %d", d.RedeemedCount, maxRedemptions-1)
	}
}

func TestReserveDiscountRespectsPerWorkspaceLimit(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	r := repository.NewRepository(pool)
	ws, sessions := seedCheckouts(t, pool, 2)

	discountID, err := r.CreateDiscount(ctx, gen.CreateDiscountParams{
		Code: "once" + uuid.NewString()[:6], Name: "Once", Kind: "percent", PercentBps: ptr(int32(2000)),
		Duration: "once", AppliesTo: "all", AllowInstallments: true, PerWorkspaceLimit: 1,
		StartsAt: time.Now().Add(-time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	reserve := func(s uuid.UUID) error {
		_, err := r.ReserveDiscount(ctx, gen.ReserveDiscountParams{
			WorkspaceID: ws, CheckoutSessionID: s, AmountMinor: 6000, DiscountID: discountID, Now: time.Now(),
		})
		return err
	}
	if err := reserve(sessions[0]); err != nil {
		t.Fatal(err)
	}
	if err := reserve(sessions[1]); !repository.IsNoRows(err) {
		t.Fatalf("second use in the same workspace: err = %v, want no rows", err)
	}
}

func ptr[T any](v T) *T { return &v }
