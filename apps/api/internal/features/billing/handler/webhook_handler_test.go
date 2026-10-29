package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/service"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/httpx"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

type countingQueue struct{ n int }

func (q *countingQueue) InsertTx(context.Context, pgx.Tx, river.JobArgs, *river.InsertOpts) (*rivertype.JobInsertResult, error) {
	q.n++
	return &rivertype.JobInsertResult{Job: &rivertype.JobRow{}}, nil
}

func TestReceiveWebhookRoute(t *testing.T) {
	httpx.InstallErrorHandler()
	pool := dbtest.New(t)
	fake := payment.NewFake(payment.Stripe, "whsec_test")
	reg, err := payment.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	q := &countingQueue{}
	svc := service.New(pool, reg, q, service.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, api := humatest.New(t)
	pass := func(ctx huma.Context, next func(huma.Context)) { next(ctx) }
	Register(api, NewHandler(svc), Guards{Auth: pass, Member: pass, Can: func(...authctx.Permission) Middleware { return pass }}, nil)

	header, body, err := fake.Webhook(payment.Event{
		ExternalID: "evt_" + uuid.NewString(), Type: payment.EventRefundUpdated, ProviderType: "charge.refunded",
		OccurredAt: time.Now().UTC(), Refund: &payment.Refund{ExternalID: "re_1", ExternalPaymentID: "pay_1", AmountMinor: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	post := func(provider string, h http.Header, b []byte) (int, string) {
		args := []any{bytes.NewReader(b)}
		for name := range h {
			args = append(args, name+": "+h.Get(name))
		}
		resp := api.Post("/v1/webhooks/payments/"+provider, args...)
		var e struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(resp.Body.Bytes(), &e)
		return resp.Code, e.Code
	}

	if code, _ := post("stripe", header, body); code != http.StatusOK {
		t.Errorf("signed webhook: %d, want 200", code)
	}
	if code, _ := post("stripe", header, body); code != http.StatusOK {
		t.Errorf("replayed webhook: %d, want 200", code)
	}
	if q.n != 1 {
		t.Errorf("enqueued %d jobs, want 1", q.n)
	}
	tampered := []byte(strings.Replace(string(body), `"re_1"`, `"re_2"`, 1))
	if code, errCode := post("stripe", header, tampered); code != http.StatusBadRequest || errCode != "invalid_signature" {
		t.Errorf("bad signature: %d %q, want 400 invalid_signature", code, errCode)
	}
	if code, errCode := post("creem", header, body); code != http.StatusNotFound || errCode != "payment_provider_not_available" {
		t.Errorf("provider not enabled: %d %q, want 404 payment_provider_not_available", code, errCode)
	}
	if q.n != 1 {
		t.Errorf("rejected webhooks enqueued jobs: %d, want 1", q.n)
	}
}
