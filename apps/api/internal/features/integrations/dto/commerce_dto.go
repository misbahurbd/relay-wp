package dto

import (
	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

type ShopifyWebhookInput struct {
	ConnectionID uuid.UUID `path:"connectionId"`
	Topic        string    `header:"X-Shopify-Topic"`
	DeliveryID   string    `header:"X-Shopify-Webhook-Id"`
	Signature    string    `header:"X-Shopify-Hmac-Sha256"`
	RawBody      []byte
}

func (in *ShopifyWebhookInput) Delivery() model.CommerceDelivery {
	return model.CommerceDelivery{Topic: in.Topic, DeliveryID: in.DeliveryID, Signature: in.Signature, Body: in.RawBody}
}

type WooWebhookInput struct {
	ConnectionID uuid.UUID `path:"connectionId"`
	Topic        string    `header:"X-WC-Webhook-Topic"`
	DeliveryID   string    `header:"X-WC-Webhook-Delivery-ID"`
	Signature    string    `header:"X-WC-Webhook-Signature"`
	RawBody      []byte
}

type ShopifyPrivacyInput struct {
	Topic     string `header:"X-Shopify-Topic"`
	Signature string `header:"X-Shopify-Hmac-Sha256"`
	RawBody   []byte
}
