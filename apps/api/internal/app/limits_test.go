package app

import (
	"net/http"
	"strings"
	"testing"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	billingmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	broadcastsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	contactsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	integrationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	teammodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/middleware"
)

// Features name billing resources by key without importing billing, so a typo
// would only show up as an error on the first invite or connect.
func TestPlanResourceKeysAreInTheCatalog(t *testing.T) {
	for _, key := range []string{teammodel.PlanResourceMembers, whatsappmodel.PlanResourceNumbers, contactsmodel.PlanResourceContacts, automationsmodel.PlanResource, broadcastsmodel.PlanResource, middleware.APICallsResource,
		teammodel.PlanResourceTeamControls, integrationsmodel.PlanResourceEcommerce,
	} {
		r, ok := billingmodel.ResourceByKey(key)
		if !ok {
			t.Errorf("%q is not in billing's resource catalog", key)
		} else if !r.Enforced {
			t.Errorf("%q is enforced by its feature; mark it Enforced in the catalog", key)
		}
	}
}

// includeInPlans puts access resource key in every plan for one test.
func includeInPlans(e *authEnv, key string) {
	e.exec(`UPDATE plans SET limits = limits || jsonb_build_object($1::text, 1)`, key)
	e.t.Cleanup(func() { e.exec(`UPDATE plans SET limits = limits - $1::text`, key) })
}

// Store connections need ecommerce; custom roles and permission overrides need
// team_controls. Reading what exists doesn't.
func TestEcommerceAndTeamControlsNeedThePlan(t *testing.T) {
	e := newAuthEnv(t)
	w := newTeamWorkspace(e)
	_, agentID := w.join("agent")
	conn := "/v1/workspace/integrations/connections/"
	for _, c := range []struct {
		method, path string
		body         map[string]any
		key          string
	}{
		{"POST", w.base + "/roles", map[string]any{"name": "Shift lead", "permissions": []string{"inbox.reply"}}, "team_controls"},
		{"PATCH", w.base + "/roles/" + w.roles["agent"], map[string]any{"name": "Agent+"}, "team_controls"},
		{"POST", w.base + "/members/" + agentID + "/overrides", map[string]any{"permission_key": "team.invite", "effect": "grant"}, "team_controls"},
		{"PUT", conn + "shopify", map[string]any{"shop_domain": "greenleaf", "access_token": "good"}, "ecommerce"},
		{"PUT", conn + "woocommerce", map[string]any{"store_url": "https://shop.example", "consumer_key": "ck", "consumer_secret": "cs"}, "ecommerce"},
		{"POST", conn + "shopify/authorize", map[string]any{"shop_domain": "greenleaf"}, "ecommerce"},
		{"POST", conn + "woocommerce/authorize", map[string]any{"store_url": "https://shop.example"}, "ecommerce"},
	} {
		got := w.owner.do(c.method, c.path, c.body)
		expect(t, got, http.StatusPaymentRequired, "plan_required")
		if !strings.Contains(got.raw, c.key) {
			t.Errorf("%s %s names the wrong resource: %s", c.method, c.path, got.raw)
		}
	}
	expect(t, w.owner.do("GET", w.base+"/roles", nil), http.StatusOK)
	expect(t, w.owner.do("GET", "/v1/workspace/integrations", nil), http.StatusOK)
}
