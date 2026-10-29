package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/authctx"
)

const invoicesPath = "/v1/workspace/billing/invoices"

// invoiceKeys is every field of dto.InvoiceDTO in a list; a single invoice adds bill_to.
var invoiceKeys = []string{
	"id", "number", "subscription_id", "order_id", "installment_id", "provider", "status", "subtotal_minor",
	"discount_minor", "tax_minor", "total_minor", "currency", "period_start", "period_end", "receipt_url",
	"paid_at", "created_at",
}

// seedInvoices makes a workspace with a subscription and n paid invoices, one
// minute apart, oldest first. It returns the workspace and the invoice IDs.
func (e *env) seedInvoices(n int) (uuid.UUID, []uuid.UUID) {
	e.t.Helper()
	ctx := context.Background()
	ws := uuid.New()
	exec := func(sql string, args ...any) {
		e.t.Helper()
		if _, err := e.pool.Exec(ctx, sql, args...); err != nil {
			e.t.Fatal(err)
		}
	}
	exec(`INSERT INTO workspaces (id, name) VALUES ($1, 'Invoices test')`, ws)
	var priceID, customerID, subID uuid.UUID
	if err := e.pool.QueryRow(ctx, `
		WITH p AS (INSERT INTO plans (code, name) VALUES ($1, 'Test') RETURNING id)
		INSERT INTO plan_prices (plan_id, billing_type, interval, currency, amount_minor)
		SELECT id, 'recurring', 'month', 'USD', 2900 FROM p RETURNING id`,
		"inv_"+uuid.NewString()[:8]).Scan(&priceID); err != nil {
		e.t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `INSERT INTO billing_customers (workspace_id, provider) VALUES ($1, 'stripe') RETURNING id`,
		ws).Scan(&customerID); err != nil {
		e.t.Fatal(err)
	}
	if err := e.pool.QueryRow(ctx, `
		INSERT INTO subscriptions (workspace_id, plan_price_id, billing_customer_id, provider, status)
		VALUES ($1, $2, $3, 'stripe', 'active') RETURNING id`, ws, priceID, customerID).Scan(&subID); err != nil {
		e.t.Fatal(err)
	}
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	ids := make([]uuid.UUID, n)
	for i := range ids {
		at := start.Add(time.Duration(i) * time.Minute)
		if err := e.pool.QueryRow(ctx, `
			INSERT INTO invoices (workspace_id, subscription_id, provider, status, subtotal_minor, total_minor,
			                      currency, bill_to, paid_at, created_at)
			VALUES ($1, $2, 'stripe', 'paid', 2900, 2900, 'USD',
			        '{"legal_name": "Acme Ltd", "email": "billing@acme.test"}', $3, $3)
			RETURNING id`, ws, subID, at).Scan(&ids[i]); err != nil {
			e.t.Fatal(err)
		}
	}
	return ws, ids
}

func TestInvoicesNeedBillingView(t *testing.T) {
	e := newEnv(t, nil)
	m := e.member(uuid.New(), authctx.PermBillingPay)
	expectDenied(t, m.do(http.MethodGet, invoicesPath, nil), authctx.PermBillingView)
	expectDenied(t, m.do(http.MethodGet, invoicesPath+"/"+uuid.NewString(), nil), authctx.PermBillingView)
}

func TestListInvoices(t *testing.T) {
	e := dbEnv(t)
	ws, ids := e.seedInvoices(model.InvoicePageSize + 2)
	other, _ := e.seedInvoices(1)
	m := e.viewer(ws)

	first := m.do(http.MethodGet, invoicesPath, nil)
	expect(t, first, http.StatusOK)
	delete(first.body, "$schema") // humatest's default config links the schema
	expectKeys(t, first.body, "invoices", "next_cursor")
	page, _ := first.body["invoices"].([]any)
	if len(page) != model.InvoicePageSize {
		t.Fatalf("first page has %d invoices, want %d", len(page), model.InvoicePageSize)
	}
	newest := page[0].(map[string]any)
	expectKeys(t, newest, invoiceKeys...)
	if newest["id"] != ids[len(ids)-1].String() || newest["total_minor"] != float64(2900) || newest["status"] != "paid" {
		t.Fatalf("first invoice %v, want the newest one (%s)", newest, ids[len(ids)-1])
	}
	cursor, ok := first.body["next_cursor"].(string)
	if !ok || cursor == "" {
		t.Fatalf("next_cursor %v, want a cursor", first.body["next_cursor"])
	}

	second := m.do(http.MethodGet, invoicesPath+"?cursor="+cursor, nil)
	expect(t, second, http.StatusOK)
	rest, _ := second.body["invoices"].([]any)
	if len(rest) != 2 || rest[1].(map[string]any)["id"] != ids[0].String() {
		t.Fatalf("second page %v, want the two oldest invoices", rest)
	}
	if second.body["next_cursor"] != nil {
		t.Fatalf("next_cursor %v on the last page, want null", second.body["next_cursor"])
	}

	// Another workspace sees only its own.
	theirs := e.viewer(other).do(http.MethodGet, invoicesPath, nil)
	expect(t, theirs, http.StatusOK)
	if got := theirs.body["invoices"].([]any); len(got) != 1 {
		t.Fatalf("other workspace lists %d invoices, want 1", len(got))
	}

	expect(t, m.do(http.MethodGet, invoicesPath+"?cursor=garbage", nil), http.StatusUnprocessableEntity, "invalid_cursor")
}

func TestListInvoicesEmpty(t *testing.T) {
	e := dbEnv(t)
	ws, _ := e.seedInvoices(0)
	r := e.viewer(ws).do(http.MethodGet, invoicesPath, nil)
	expect(t, r, http.StatusOK)
	if got, ok := r.body["invoices"].([]any); !ok || len(got) != 0 {
		t.Fatalf("invoices %v, want []", r.body["invoices"])
	}
}

func TestInvoice(t *testing.T) {
	e := dbEnv(t)
	ws, ids := e.seedInvoices(1)
	other, theirs := e.seedInvoices(1)
	m := e.viewer(ws)

	r := m.do(http.MethodGet, invoicesPath+"/"+ids[0].String(), nil)
	expect(t, r, http.StatusOK)
	delete(r.body, "$schema")
	expectKeys(t, r.body, append([]string{"bill_to"}, invoiceKeys...)...)
	billTo := r.body["bill_to"].(map[string]any)
	if billTo["legal_name"] != "Acme Ltd" || billTo["email"] != "billing@acme.test" {
		t.Fatalf("bill_to %v, want the details stored on the invoice", billTo)
	}

	// Another workspace's invoice and an unknown ID look the same.
	expect(t, m.do(http.MethodGet, invoicesPath+"/"+theirs[0].String(), nil), http.StatusNotFound, "invoice_not_found")
	expect(t, m.do(http.MethodGet, invoicesPath+"/"+uuid.NewString(), nil), http.StatusNotFound, "invoice_not_found")
	expect(t, e.viewer(other).do(http.MethodGet, invoicesPath+"/"+theirs[0].String(), nil), http.StatusOK)
}
