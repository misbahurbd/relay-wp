package dto

import "github.com/misbahurbd/relay-wp/apps/api/internal/features/billing/model"

type CardDTO struct {
	Brand    string `json:"brand" example:"visa" doc:"The card network, lower case"`
	Last4    string `json:"last4" example:"4242"`
	ExpMonth int    `json:"exp_month" example:"8"`
	ExpYear  int    `json:"exp_year" example:"2028"`
}

type PaymentMethodOutput struct {
	Body struct {
		Provider    *string  `json:"provider,omitempty" enum:"stripe,creem,paypal,sslcommerz" doc:"Absent until something is bought"`
		Card        *CardDTO `json:"card,omitempty" doc:"Read from the provider each time; absent when there is none, or the provider shares no card details"`
		CardDetails bool     `json:"card_details" doc:"False when the provider never shares card details (Creem)"`
		CanManage   bool     `json:"can_manage" doc:"The provider has a portal for changing cards"`
	}
}

func ToPaymentMethodOutput(m model.PaymentMethod) *PaymentMethodOutput {
	out := &PaymentMethodOutput{}
	out.Body.Provider = nonEmpty(m.Provider)
	out.Body.CardDetails, out.Body.CanManage = m.CardDetails, m.CanManage
	if c := m.Card; c != nil {
		out.Body.Card = &CardDTO{Brand: c.Brand, Last4: c.Last4, ExpMonth: c.ExpMonth, ExpYear: c.ExpYear}
	}
	return out
}

type PortalSessionOutput struct {
	Body struct {
		URL string `json:"url" format:"uri" doc:"The provider's card management page; short-lived, open it at once"`
	}
}
