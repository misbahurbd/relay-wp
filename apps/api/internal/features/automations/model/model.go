// Package model holds automations' domain types: the six preset kinds, their
// typed settings and errors.
package model

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/shared/apperr"
)

// PlanResource is the billing resource that gates automations (ADR 0008).
const PlanResource = "automations"

// Kind is one of the six preset automations.
type Kind string

// The kinds, in the order the page shows them.
const (
	KindWelcome Kind = "welcome"
	KindAway    Kind = "away"
	KindKeyword Kind = "keyword"
	KindCart    Kind = "cart"
	KindOrder   Kind = "order"
	KindDrip    Kind = "drip"
)

// Kinds lists every kind in page order.
var Kinds = []Kind{KindWelcome, KindAway, KindKeyword, KindCart, KindOrder, KindDrip}

// Settings is a kind's configuration.
type Settings interface {
	// Validate returns what is wrong with values that were entered.
	Validate() []apperr.Detail
	// Ready returns what is still missing before the automation can run.
	Ready() []apperr.Detail
}

// Automation is one kind in a workspace. UpdatedAt is nil until it is first saved.
type Automation struct {
	Kind      Kind
	Enabled   bool
	Settings  Settings
	UpdatedAt *time.Time
	// Sent counts messages sent this month.
	Sent int64
}

// Default is a kind's settings before anyone edits them.
func Default(k Kind) Settings {
	switch k {
	case KindWelcome:
		return defaultWelcome()
	case KindAway:
		return defaultAway()
	case KindKeyword:
		return defaultKeyword()
	case KindCart:
		return defaultCart()
	case KindOrder:
		return defaultOrder()
	case KindDrip:
		return defaultDrip()
	}
	panic(fmt.Sprintf("automations: unknown kind %q", k))
}

// Decode reads a kind's stored settings.
func Decode(k Kind, raw []byte) (Settings, error) {
	s := Default(k)
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("decode %s settings: %w", k, err)
	}
	return s, nil
}
