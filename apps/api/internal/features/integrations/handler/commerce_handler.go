package handler

import (
	"context"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

func (h *Handler) shopifyWebhook(ctx context.Context, in *dto.ShopifyWebhookInput) (*struct{}, error) {
	return nil, h.svc.CommerceWebhook(ctx, model.ProviderShopify, in.ConnectionID, in.Delivery())
}

func (h *Handler) wooWebhook(ctx context.Context, in *dto.WooWebhookInput) (*struct{}, error) {
	return nil, h.svc.CommerceWebhook(ctx, model.ProviderWooCommerce, in.ConnectionID, model.CommerceDelivery{
		Topic: in.Topic, DeliveryID: in.DeliveryID, Signature: in.Signature, Body: in.RawBody,
	})
}

func (h *Handler) shopifyPrivacy(ctx context.Context, in *dto.ShopifyPrivacyInput) (*struct{}, error) {
	return nil, h.svc.ShopifyPrivacy(ctx, model.CommerceDelivery{Topic: in.Topic, Signature: in.Signature, Body: in.RawBody})
}
