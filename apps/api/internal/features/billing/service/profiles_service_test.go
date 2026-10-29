package service

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
)

func TestProfileUpdateAndBillTo(t *testing.T) {
	e := newT3Env(t)
	ws := e.workspace()
	if _, err := e.svc.Profile(e.ctx, ws); !errors.Is(err, model.ErrProfileNotFound) {
		t.Fatalf("no profile: %v", err)
	}
	blank, country := "  ", "bd"
	if _, err := e.svc.UpdateProfile(e.ctx, ws, model.ProfileInput{LegalName: " ", Email: "nope"}); !t3IsInvalidInput(err) {
		t.Fatalf("invalid profile: %v", err)
	}
	taxID := "123"
	if _, err := e.svc.UpdateProfile(e.ctx, ws, model.ProfileInput{LegalName: "A", Email: "a@b.test", TaxID: &taxID}); !t3IsInvalidInput(err) {
		t.Fatalf("tax id without type: %v", err)
	}
	p, err := e.svc.UpdateProfile(e.ctx, ws, model.ProfileInput{
		LegalName: " Acme ", Email: "billing@acme.test", Phone: &blank, Country: &country,
	})
	if err != nil || p.LegalName != "Acme" || p.Phone != nil || p.Country == nil || *p.Country != "BD" {
		t.Fatalf("profile = %+v, %v", p, err)
	}
	e.inTx(func(tx pgx.Tx) error {
		b, err := e.svc.BillTo(e.ctx, tx, ws)
		if b.LegalName != "Acme" || b.Email != "billing@acme.test" || *b.Country != "BD" {
			t.Errorf("bill to = %+v", b)
		}
		return err
	})
}
