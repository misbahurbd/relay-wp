package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
)

// Overview gathers what each card on the Integrations page shows.
func (s *Service) Overview(ctx context.Context, workspaceID uuid.UUID) (model.Overview, error) {
	conns, err := s.Connections(ctx, workspaceID)
	if err != nil {
		return model.Overview{}, err
	}
	keys, err := s.APIKeys(ctx, workspaceID)
	if err != nil {
		return model.Overview{}, err
	}
	endpoints, err := s.Endpoints(ctx, workspaceID)
	if err != nil {
		return model.Overview{}, err
	}
	out := model.Overview{
		Connections: conns, GoogleAvailable: s.cfg.Google.ClientID != "",
		ShopifyOAuth: s.cfg.Shopify.ClientID != "", WooOAuth: s.WooOAuthAvailable(), ActiveKeys: len(keys),
		AppKeys: map[string]model.AppKeyStatus{}, Endpoints: endpoints,
	}
	for _, k := range keys {
		if k.LastUsedAt != nil && (out.KeysLastUsedAt == nil || k.LastUsedAt.After(*out.KeysLastUsedAt)) {
			out.KeysLastUsedAt = k.LastUsedAt
		}
		if k.App == nil {
			continue
		}
		st := out.AppKeys[*k.App]
		st.ActiveKeys++
		if k.LastUsedAt != nil && (st.LastUsedAt == nil || k.LastUsedAt.After(*st.LastUsedAt)) {
			st.LastUsedAt = k.LastUsedAt
		}
		out.AppKeys[*k.App] = st
	}
	return out, nil
}
