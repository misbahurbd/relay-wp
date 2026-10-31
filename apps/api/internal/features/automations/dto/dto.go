// Package dto holds automations' HTTP request and response shapes and the converters from domain types.
package dto

import (
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/automations/model"
)

type WorkspaceInput struct {
	WorkspaceID uuid.UUID `header:"X-Workspace-ID" required:"true"`
}

// SettingsDTO holds one kind's settings, in the field named after the kind.
type SettingsDTO struct {
	Welcome *model.WelcomeSettings `json:"welcome,omitempty"`
	Away    *model.AwaySettings    `json:"away,omitempty"`
	Keyword *model.KeywordSettings `json:"keyword,omitempty"`
	Cart    *model.CartSettings    `json:"cart,omitempty"`
	Order   *model.OrderSettings   `json:"order,omitempty"`
	Drip    *model.DripSettings    `json:"drip,omitempty"`
}

// For returns the settings for kind, or nil when that field wasn't sent.
func (s SettingsDTO) For(kind model.Kind) model.Settings {
	switch {
	case kind == model.KindWelcome && s.Welcome != nil:
		return s.Welcome
	case kind == model.KindAway && s.Away != nil:
		return s.Away
	case kind == model.KindKeyword && s.Keyword != nil:
		return s.Keyword
	case kind == model.KindCart && s.Cart != nil:
		return s.Cart
	case kind == model.KindOrder && s.Order != nil:
		return s.Order
	case kind == model.KindDrip && s.Drip != nil:
		return s.Drip
	}
	return nil
}

type AutomationDTO struct {
	Kind              string      `json:"kind" enum:"welcome,away,keyword,cart,order,drip"`
	Enabled           bool        `json:"enabled"`
	Settings          SettingsDTO `json:"settings"`
	UpdatedAt         *time.Time  `json:"updated_at,omitempty" doc:"Absent until the automation is first saved"`
	Sent              int64       `json:"sent" doc:"Messages sent this month"`
	Failed            int64       `json:"failed" doc:"Messages that could not be sent this month"`
	Read              int64       `json:"read" doc:"Messages read this month"`
	Active            int64       `json:"active" doc:"Contacts still waiting in this automation"`
	Recovered         int64       `json:"recovered" doc:"Carts bought after a reminder this month"`
	RecoveredMinor    int64       `json:"recovered_minor" doc:"What those carts were worth, in minor units"`
	RecoveredCurrency string      `json:"recovered_currency" doc:"Their currency"`
}

func ToAutomationDTO(a model.Automation) AutomationDTO {
	out := AutomationDTO{
		Kind: string(a.Kind), Enabled: a.Enabled, UpdatedAt: a.UpdatedAt, Sent: a.Stats.Sent, Failed: a.Stats.Failed, Read: a.Stats.Read, Active: a.Stats.Active,
		Recovered: a.Stats.Recovered, RecoveredMinor: a.Stats.RecoveredMinor, RecoveredCurrency: a.Stats.RecoveredCurrency,
	}
	switch s := a.Settings.(type) {
	case *model.WelcomeSettings:
		out.Settings.Welcome = s
	case *model.AwaySettings:
		out.Settings.Away = s
	case *model.KeywordSettings:
		out.Settings.Keyword = s
	case *model.CartSettings:
		out.Settings.Cart = s
	case *model.OrderSettings:
		out.Settings.Order = s
	case *model.DripSettings:
		out.Settings.Drip = s
	}
	return out
}

type ListOutput struct {
	Body struct {
		Items []AutomationDTO `json:"items" doc:"All six kinds in page order"`
	}
}

type AutomationOutput struct {
	Body AutomationDTO
}

type SaveInput struct {
	WorkspaceInput
	Kind string `path:"kind" enum:"welcome,away,keyword,cart,order,drip"`
	Body SettingsDTO
}

type SetEnabledInput struct {
	WorkspaceInput
	Kind string `path:"kind" enum:"welcome,away,keyword,cart,order,drip"`
	Body struct {
		Enabled bool `json:"enabled"`
	}
}
