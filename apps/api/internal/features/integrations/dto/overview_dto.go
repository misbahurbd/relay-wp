package dto

import (
	"time"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

type keysSummaryDTO struct {
	Active     int        `json:"active"`
	LastUsedAt *time.Time `json:"last_used_at" nullable:"true"`
}

type appKeysDTO struct {
	App        string     `json:"app" enum:"zapier,make,n8n"`
	ActiveKeys int        `json:"active_keys"`
	LastUsedAt *time.Time `json:"last_used_at" nullable:"true"`
}

type webhooksSummaryDTO struct {
	Endpoints     int        `json:"endpoints"`
	Enabled       int        `json:"enabled"`
	WeekTotal     int64      `json:"week_total"`
	WeekSucceeded int64      `json:"week_succeeded"`
	LastStatus    *string    `json:"last_status" nullable:"true" doc:"succeeded, failed or pending"`
	LastResponse  *int32     `json:"last_response_status" nullable:"true"`
	LastDelivery  *time.Time `json:"last_delivery_at" nullable:"true"`
}

type IntegrationsOverviewOutput struct {
	Body struct {
		Connections     []ConnectionDTO    `json:"connections"`
		GoogleAvailable bool               `json:"google_available" doc:"False: this server has no Google app, so Sheets can't connect"`
		ShopifyOAuth    bool               `json:"shopify_oauth" doc:"True: Shopify connects in one click; false: with a custom app's access token"`
		WooOAuth        bool               `json:"woocommerce_oauth" doc:"True: WooCommerce connects in one click; false: with REST API keys"`
		APIKeys         keysSummaryDTO     `json:"api_keys"`
		Apps            []appKeysDTO       `json:"apps" doc:"No-code apps with at least one active key"`
		Webhooks        webhooksSummaryDTO `json:"webhooks"`
	}
}

func ToIntegrationsOverviewOutput(o model.Overview) *IntegrationsOverviewOutput {
	out := &IntegrationsOverviewOutput{}
	b := &out.Body
	b.Connections = make([]ConnectionDTO, 0, len(o.Connections))
	for _, c := range o.Connections {
		b.Connections = append(b.Connections, ToConnectionDTO(c))
	}
	b.GoogleAvailable, b.ShopifyOAuth, b.WooOAuth = o.GoogleAvailable, o.ShopifyOAuth, o.WooOAuth
	b.APIKeys = keysSummaryDTO{Active: o.ActiveKeys, LastUsedAt: o.KeysLastUsedAt}
	b.Apps = make([]appKeysDTO, 0, len(o.AppKeys))
	for _, app := range model.KeyApps {
		if st, ok := o.AppKeys[app]; ok {
			b.Apps = append(b.Apps, appKeysDTO{App: app, ActiveKeys: st.ActiveKeys, LastUsedAt: st.LastUsedAt})
		}
	}
	w := &b.Webhooks
	w.Endpoints = len(o.Endpoints)
	for _, e := range o.Endpoints {
		if e.Enabled {
			w.Enabled++
		}
		w.WeekTotal += e.WeekTotal
		w.WeekSucceeded += e.WeekSucceeded
		if e.LastDeliveryAt != nil && (w.LastDelivery == nil || e.LastDeliveryAt.After(*w.LastDelivery)) {
			w.LastDelivery, w.LastStatus, w.LastResponse = e.LastDeliveryAt, e.LastStatus, e.LastResponseStatus
		}
	}
	return out
}
