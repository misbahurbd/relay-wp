package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

func TestRunBillingRejectsUnknownCommands(t *testing.T) {
	for _, args := range [][]string{nil, {"discounts"}, {"plans", "list"}} {
		err := runBilling(context.Background(), config.Config{}, nil, args, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "usage:") {
			t.Errorf("runBilling(%q) = %v, want usage", args, err)
		}
	}
}

func TestDiscountOff(t *testing.T) {
	bps, amount, usd := 1250, int64(500), "USD"
	if got := discountOff(model.Discount{PercentBps: &bps}); got != "12.5%" {
		t.Errorf("percent = %q", got)
	}
	if got := discountOff(model.Discount{AmountOffMinor: &amount, Currency: &usd}); got != "500 USD minor" {
		t.Errorf("amount = %q", got)
	}
}

func TestCliErrSpellsOutDetails(t *testing.T) {
	err := cliErr(model.ErrInvalidInput.WithDetails(apperr.Detail{Field: "code", Message: "too short"}))
	if !strings.Contains(err.Error(), "code: too short") {
		t.Errorf("cliErr = %v", err)
	}
	plain := errors.New("boom")
	if got := cliErr(plain); !errors.Is(got, plain) {
		t.Errorf("cliErr(plain) = %v", got)
	}
}
