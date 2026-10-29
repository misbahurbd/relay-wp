package model_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/payment"
)

func TestSplitInstallmentsAddsUpExactly(t *testing.T) {
	for _, tt := range []struct {
		total int64
		count int
		want  []int64
	}{
		{30000, 3, []int64{10000, 10000, 10000}},
		{10000, 3, []int64{3333, 3333, 3334}},
		{29999, 2, []int64{14999, 15000}},
	} {
		got := model.SplitInstallments(tt.total, tt.count)
		var sum int64
		for i, v := range got {
			sum += v
			if v != tt.want[i] {
				t.Errorf("SplitInstallments(%d, %d) = %v, want %v", tt.total, tt.count, got, tt.want)
				break
			}
		}
		if sum != tt.total {
			t.Errorf("SplitInstallments(%d, %d) adds up to %d", tt.total, tt.count, sum)
		}
	}
	if model.SplitInstallments(100, 0) != nil {
		t.Error("count 0 should give no installments")
	}
}

func TestParseLimitsTreatsMissingKeysAsUnlimited(t *testing.T) {
	l, err := model.ParseLimits([]byte(`{"members": 10}`))
	if err != nil {
		t.Fatal(err)
	}
	if l.Members != 10 || l.WhatsAppNumbers != model.Unlimited {
		t.Errorf("limits = %+v", l)
	}
	if _, err := model.ParseLimits([]byte(`[]`)); err == nil {
		t.Error("a JSON array should not parse")
	}
}

func TestProcessPaymentEventArgsLeaveThePayloadOut(t *testing.T) {
	args := model.ProcessPaymentEventArgs{Event: payment.Event{ExternalID: "evt_1", Payload: []byte(`{"secret":"x"}`)}}
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "secret") {
		t.Errorf("job args carry the payload: %s", b)
	}
}
