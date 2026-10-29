package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	billingservice "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/config"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

const billingDiscountsUsage = `usage:
  relay billing discounts create -code CODE (-percent P | -amount MINOR -currency USD) [flags]
  relay billing discounts list [-all]
  relay billing discounts deactivate CODE`

// runBilling runs `relay billing ...`: today only the discounts commands (Q18).
func runBilling(ctx context.Context, cfg config.Config, log *slog.Logger, args []string, out io.Writer) error {
	if len(args) < 2 || args[0] != "discounts" {
		return errors.New(billingDiscountsUsage)
	}
	billing, closeFn, err := newBillingService(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer closeFn()

	switch cmd, rest := args[1], args[2:]; cmd {
	case "create":
		return createDiscount(ctx, billing, rest, out)
	case "list":
		return listDiscounts(ctx, billing, rest, out)
	case "deactivate":
		if len(rest) != 1 {
			return errors.New(billingDiscountsUsage)
		}
		if err := billing.DeactivateDiscount(ctx, rest[0]); err != nil {
			return cliErr(err)
		}
		return printf(out, "deactivated %s\n", strings.ToUpper(rest[0]))
	default:
		return fmt.Errorf("unknown discounts command %q\n%s", cmd, billingDiscountsUsage)
	}
}

// newBillingService builds the billing service the way internal/app does,
// without a job queue: these commands enqueue nothing.
func newBillingService(ctx context.Context, cfg config.Config, log *slog.Logger) (*billingservice.Service, func(), error) {
	payments, err := payment.New(cfg.Payment, cfg.IsDevelopment())
	if err != nil {
		return nil, nil, fmt.Errorf("payment providers: %w", err)
	}
	pool, err := database.New(ctx, cfg.Database.URL, 2)
	if err != nil {
		return nil, nil, fmt.Errorf("database: %w", err)
	}
	svc := billingservice.New(pool, payments, nil, billingservice.Config{AppBaseURL: cfg.AppBaseURL}, log)
	return svc, pool.Close, nil
}

func createDiscount(ctx context.Context, billing *billingservice.Service, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("discounts create", flag.ContinueOnError)
	fs.SetOutput(out)
	var (
		code, name, currency, planID, starts, expires string
		percent                                       string
		amount                                        int64
		duration, appliesTo                           string
		months, maxRedemptions, perWorkspace          int
		noInstallments                                bool
	)
	fs.StringVar(&code, "code", "", "the code customers type (3 to 40 letters, digits, - or _)")
	fs.StringVar(&name, "name", "", "a name for reports (default: the code)")
	fs.StringVar(&percent, "percent", "", "percent off, e.g. 20 or 12.5")
	fs.Int64Var(&amount, "amount", 0, "amount off in minor units (cents); needs -currency")
	fs.StringVar(&currency, "currency", "", "currency of -amount, e.g. USD")
	fs.StringVar(&duration, "duration", model.DurationOnce, "on subscriptions: once, repeating or forever")
	fs.IntVar(&months, "months", 0, "how many months a repeating discount lasts")
	fs.StringVar(&appliesTo, "applies-to", model.AppliesToAll, "all, recurring or one_time prices")
	fs.StringVar(&planID, "plan", "", "only this plan's ID")
	fs.BoolVar(&noInstallments, "no-installments", false, "not for installment offers")
	fs.IntVar(&maxRedemptions, "max", 0, "total uses across all workspaces (0: no cap)")
	fs.IntVar(&perWorkspace, "per-workspace", 1, "uses per workspace")
	fs.StringVar(&starts, "starts", "", "when it starts, RFC 3339 (default: now)")
	fs.StringVar(&expires, "expires", "", "when it expires, RFC 3339 (default: never)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments %q\n%s", fs.Args(), billingDiscountsUsage)
	}

	in := model.NewDiscount{
		Code: code, Name: name, Duration: duration, AppliesTo: appliesTo, AllowInstallments: !noInstallments,
		PerWorkspaceLimit: perWorkspace,
	}
	switch {
	case percent != "" && amount != 0:
		return errors.New("give -percent or -amount, not both")
	case percent != "":
		p, err := strconv.ParseFloat(percent, 64)
		if err != nil {
			return fmt.Errorf("-percent: %w", err)
		}
		in.Kind, in.PercentBps = model.DiscountPercent, ptr(int(math.Round(p*100)))
	case amount != 0:
		in.Kind, in.AmountOffMinor, in.Currency = model.DiscountAmount, &amount, &currency
	default:
		return errors.New("give -percent or -amount")
	}
	if months != 0 {
		in.DurationMonths = &months
	}
	if maxRedemptions != 0 {
		in.MaxRedemptions = &maxRedemptions
	}
	if planID != "" {
		id, err := uuid.Parse(planID)
		if err != nil {
			return fmt.Errorf("-plan: %w", err)
		}
		in.PlanID = &id
	}
	var err error
	if in.StartsAt, err = parseTime(starts); err != nil {
		return fmt.Errorf("-starts: %w", err)
	}
	if expires != "" {
		t, err := parseTime(expires)
		if err != nil {
			return fmt.Errorf("-expires: %w", err)
		}
		in.ExpiresAt = &t
	}

	d, err := billing.CreateDiscount(ctx, in)
	if err != nil {
		return cliErr(err)
	}
	return printf(out, "created %s (%s)\n", d.Code, d.ID)
}

func listDiscounts(ctx context.Context, billing *billingservice.Service, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("discounts list", flag.ContinueOnError)
	fs.SetOutput(out)
	all := fs.Bool("all", false, "include deactivated codes")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	discounts, err := billing.ListDiscounts(ctx, !*all)
	if err != nil {
		return cliErr(err)
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if err := printf(w, "CODE\tOFF\tDURATION\tAPPLIES TO\tUSED\tEXPIRES\tACTIVE\n"); err != nil {
		return err
	}
	for _, d := range discounts {
		used := strconv.Itoa(d.RedeemedCount)
		if d.MaxRedemptions != nil {
			used += "/" + strconv.Itoa(*d.MaxRedemptions)
		}
		exp := "never"
		if d.ExpiresAt != nil {
			exp = d.ExpiresAt.UTC().Format(time.DateOnly)
		}
		dur := d.Duration
		if d.DurationMonths != nil {
			dur += " " + strconv.Itoa(*d.DurationMonths) + "mo"
		}
		if err := printf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%t\n", d.Code, discountOff(d), dur, d.AppliesTo, used, exp, d.IsActive); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("write discounts: %w", err)
	}
	return nil
}

func discountOff(d model.Discount) string {
	if d.PercentBps != nil {
		return strconv.FormatFloat(float64(*d.PercentBps)/100, 'f', -1, 64) + "%"
	}
	if d.AmountOffMinor != nil && d.Currency != nil {
		return strconv.FormatInt(*d.AmountOffMinor, 10) + " " + *d.Currency + " minor"
	}
	return "?"
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time: %w", err)
	}
	return t, nil
}

func printf(out io.Writer, format string, args ...any) error {
	if _, err := fmt.Fprintf(out, format, args...); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

// cliErr spells out a validation error's field details, which the API would send as JSON.
func cliErr(err error) error {
	e, ok := apperr.As(err)
	if !ok || len(e.Details) == 0 {
		return err
	}
	msgs := make([]string, 0, len(e.Details))
	for _, d := range e.Details {
		msgs = append(msgs, d.Field+": "+d.Message)
	}
	return fmt.Errorf("%s: %s", e.Message, strings.Join(msgs, "; "))
}

func ptr[T any](v T) *T { return &v }
