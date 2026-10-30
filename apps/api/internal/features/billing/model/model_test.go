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

func TestLimitsLeaveOutWhatAPlanLacks(t *testing.T) {
	l, err := model.ParseLimits([]byte(`{"members": 10, "contacts": -1, "automations": 1}`))
	if err != nil {
		t.Fatal(err)
	}
	if l.Of("members") != 10 || l.Of("contacts") != model.Unlimited || l.Of("automations") != model.Included {
		t.Errorf("limits = %v", l)
	}
	if l.Of("whatsapp_numbers") != model.NotIncluded || l.Of("ai_replies") != model.NotIncluded {
		t.Errorf("absent keys should not be included: %v", l)
	}
	if got := l.Complete(); len(got) != len(model.Resources) || got["broadcasts"] != model.NotIncluded {
		t.Errorf("complete limits = %v", got)
	}
	if _, err := model.ParseLimits([]byte(`[]`)); err == nil {
		t.Error("a JSON array should not parse")
	}
}

func TestOverridesWinKeyByKey(t *testing.T) {
	plan := model.Limits{"members": 5, "whatsapp_numbers": 1, "automations": 1}
	got := plan.With(model.Limits{"members": 20, "automations": 0})
	if got.Of("members") != 20 || got.Of("whatsapp_numbers") != 1 || got.Of("automations") != model.NotIncluded {
		t.Errorf("merged = %v", got)
	}
	if plan.Of("members") != 5 {
		t.Error("With changed the plan's limits")
	}
}

func TestCheckPlanLimits(t *testing.T) {
	ok := model.Limits{"members": -1, "whatsapp_numbers": 3, "broadcasts": 30, "automations": 1}
	if d := ok.Check("limits"); len(d) != 0 {
		t.Errorf("valid limits refused: %v", d)
	}
	for name, l := range map[string]model.Limits{
		"unknown key":      {"members": 1, "whatsapp_numbers": 1, "memebrs": 3},
		"below -1":         {"members": -2, "whatsapp_numbers": 1},
		"feature as count": {"members": 1, "whatsapp_numbers": 1, "automations": 5},
		"no members":       {"whatsapp_numbers": 1},
	} {
		if d := l.Check("limits"); len(d) != 1 {
			t.Errorf("%s: details = %v, want one", name, d)
		}
	}
	if d := (model.Limits{"members": 0}).CheckOverrides("overrides"); len(d) != 1 {
		t.Errorf("overriding members away: details = %v", d)
	}
	if d := (model.Limits{"contacts": 0, "automations": 1}).CheckOverrides("overrides"); len(d) != 0 {
		t.Errorf("valid overrides refused: %v", d)
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
