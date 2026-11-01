package app

import (
	"testing"

	automationsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
	billingmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"
	broadcastsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/broadcasts/model"
	contactsmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/contacts/model"
	teammodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/team/model"
	whatsappmodel "github.com/misbahurbd/relay-wp/apps/api/internal/features/whatsapp/model"
)

// Features name billing resources by key without importing billing, so a typo
// would only show up as an error on the first invite or connect.
func TestPlanResourceKeysAreInTheCatalog(t *testing.T) {
	for _, key := range []string{teammodel.PlanResourceMembers, whatsappmodel.PlanResourceNumbers, contactsmodel.PlanResourceContacts, automationsmodel.PlanResource, broadcastsmodel.PlanResource} {
		r, ok := billingmodel.ResourceByKey(key)
		if !ok {
			t.Errorf("%q is not in billing's resource catalog", key)
		} else if !r.Enforced {
			t.Errorf("%q is enforced by its feature; mark it Enforced in the catalog", key)
		}
	}
}
