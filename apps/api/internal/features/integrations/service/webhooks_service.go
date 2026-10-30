package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/repository"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database"
)

// secretPrefix starts every signing secret, so people can tell it from an API key.
const secretPrefix = "whsec_"

// Endpoints lists the workspace's webhook endpoints with their recent delivery health.
func (s *Service) Endpoints(ctx context.Context, workspaceID uuid.UUID) ([]model.Endpoint, error) {
	repo := repository.NewRepository(s.pool)
	rows, err := repo.ListWebhookEndpoints(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	stats, err := repo.WebhookEndpointStats(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("webhook stats: %w", err)
	}
	byID := make(map[uuid.UUID]gen.WebhookEndpointStatsRow, len(stats))
	for _, st := range stats {
		byID[st.EndpointID] = st
	}
	out := make([]model.Endpoint, 0, len(rows))
	for _, r := range rows {
		e := model.Endpoint{
			ID: r.ID, URL: r.Url, Description: r.Description, Events: r.Events, Enabled: r.Enabled,
			ConsecutiveFailures: r.ConsecutiveFailures, DisabledReason: r.DisabledReason,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if st, ok := byID[r.ID]; ok {
			e.LastStatus, e.LastResponseStatus, e.LastDeliveryAt = &st.LastStatus, st.LastResponseStatus, &st.LastDeliveryAt
			e.WeekTotal, e.WeekSucceeded = st.WeekTotal, st.WeekSucceeded
		}
		out = append(out, e)
	}
	return out, nil
}

// Endpoint returns one endpoint with its health.
func (s *Service) Endpoint(ctx context.Context, workspaceID, endpointID uuid.UUID) (model.Endpoint, error) {
	list, err := s.Endpoints(ctx, workspaceID)
	if err != nil {
		return model.Endpoint{}, err
	}
	for _, e := range list {
		if e.ID == endpointID {
			return e, nil
		}
	}
	return model.Endpoint{}, model.ErrEndpointNotFound
}

// CreateEndpoint adds an endpoint and returns it with its new signing secret.
func (s *Service) CreateEndpoint(ctx context.Context, workspaceID, userID uuid.UUID, in model.EndpointInput) (model.Endpoint, string, error) {
	in, err := s.checkEndpoint(in)
	if err != nil {
		return model.Endpoint{}, "", err
	}
	secret, sealed, err := s.newSecret()
	if err != nil {
		return model.Endpoint{}, "", err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Endpoint{}, "", fmt.Errorf("new id: %w", err)
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		repo := repository.NewRepository(tx)
		if err := repo.LockWorkspaceIntegrations(ctx, workspaceID); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		n, err := repo.CountWebhookEndpoints(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("count webhooks: %w", err)
		}
		if n >= model.MaxEndpoints {
			return model.ErrTooManyEndpoints
		}
		return repo.CreateWebhookEndpoint(ctx, gen.CreateWebhookEndpointParams{
			ID: id, WorkspaceID: workspaceID, Url: in.URL, Description: in.Description, Events: in.Events,
			Secret: sealed, CreatedBy: &userID,
		})
	})
	if err != nil {
		return model.Endpoint{}, "", err
	}
	e, err := s.Endpoint(ctx, workspaceID, id)
	return e, secret, err
}

// UpdateEndpoint changes an endpoint's URL, description, events or on/off.
// Turning it on clears the failure streak that turned it off.
func (s *Service) UpdateEndpoint(ctx context.Context, workspaceID, endpointID uuid.UUID, in model.EndpointInput) (model.Endpoint, error) {
	in, err := s.checkEndpoint(in)
	if err != nil {
		return model.Endpoint{}, err
	}
	n, err := repository.NewRepository(s.pool).UpdateWebhookEndpoint(ctx, gen.UpdateWebhookEndpointParams{
		Url: in.URL, Description: in.Description, Events: in.Events, Enabled: in.Enabled,
		WorkspaceID: workspaceID, ID: endpointID,
	})
	if err != nil {
		return model.Endpoint{}, fmt.Errorf("update webhook: %w", err)
	}
	if n == 0 {
		return model.Endpoint{}, model.ErrEndpointNotFound
	}
	return s.Endpoint(ctx, workspaceID, endpointID)
}

// DeleteEndpoint removes an endpoint and its delivery log. Queued deliveries find nothing and stop.
func (s *Service) DeleteEndpoint(ctx context.Context, workspaceID, endpointID uuid.UUID) error {
	n, err := repository.NewRepository(s.pool).DeleteWebhookEndpoint(ctx, gen.DeleteWebhookEndpointParams{
		WorkspaceID: workspaceID, ID: endpointID,
	})
	if err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	if n == 0 {
		return model.ErrEndpointNotFound
	}
	return nil
}

// EndpointSecret returns the signing secret, for receivers to check signatures.
func (s *Service) EndpointSecret(ctx context.Context, workspaceID, endpointID uuid.UUID) (string, error) {
	sealed, err := repository.NewRepository(s.pool).GetWebhookSecret(ctx, gen.GetWebhookSecretParams{
		WorkspaceID: workspaceID, ID: endpointID,
	})
	if repository.IsNoRows(err) {
		return "", model.ErrEndpointNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get webhook secret: %w", err)
	}
	plain, err := s.box.open(sealed)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// RotateEndpointSecret replaces the signing secret. Deliveries signed from now on use the new one.
func (s *Service) RotateEndpointSecret(ctx context.Context, workspaceID, endpointID uuid.UUID) (string, error) {
	secret, sealed, err := s.newSecret()
	if err != nil {
		return "", err
	}
	n, err := repository.NewRepository(s.pool).SetWebhookSecret(ctx, gen.SetWebhookSecretParams{
		Secret: sealed, WorkspaceID: workspaceID, ID: endpointID,
	})
	if err != nil {
		return "", fmt.Errorf("rotate webhook secret: %w", err)
	}
	if n == 0 {
		return "", model.ErrEndpointNotFound
	}
	return secret, nil
}

func (s *Service) newSecret() (plain string, sealed []byte, err error) {
	tok, err := randomToken(24)
	if err != nil {
		return "", nil, err
	}
	plain = secretPrefix + tok
	sealed, err = s.box.seal([]byte(plain))
	return plain, sealed, err
}

// checkEndpoint trims and validates what a person entered.
func (s *Service) checkEndpoint(in model.EndpointInput) (model.EndpointInput, error) {
	in.URL = strings.TrimSpace(in.URL)
	in.Description = strings.TrimSpace(in.Description)
	if err := s.checkTargetURL(in.URL); err != nil {
		return in, err
	}
	events := make([]string, 0, len(in.Events))
	for _, e := range in.Events {
		if !model.IsEvent(e) {
			return in, model.ErrUnknownEvent
		}
		if !slices.Contains(events, e) {
			events = append(events, e)
		}
	}
	if len(events) == 0 {
		return in, model.ErrUnknownEvent
	}
	in.Events = events
	return in, nil
}

// checkTargetURL accepts absolute https URLs (http too when private targets are allowed)
// without credentials. Private addresses are refused again when connecting, after DNS.
func (s *Service) checkTargetURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || len(raw) > 2048 {
		return model.ErrInvalidURL
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !s.cfg.AllowPrivateTargets {
			return model.ErrInvalidURL
		}
	default:
		return model.ErrInvalidURL
	}
	if !s.cfg.AllowPrivateTargets && isBlockedHost(u.Hostname()) {
		return model.ErrInvalidURL
	}
	return nil
}

// Deliveries pages through an endpoint's delivery log, newest first.
func (s *Service) Deliveries(ctx context.Context, workspaceID, endpointID uuid.UUID, afterAt *time.Time, afterID *uuid.UUID, limit int32) ([]model.Delivery, error) {
	repo := repository.NewRepository(s.pool)
	if _, err := repo.GetWebhookEndpoint(ctx, gen.GetWebhookEndpointParams{WorkspaceID: workspaceID, ID: endpointID}); err != nil {
		if repository.IsNoRows(err) {
			return nil, model.ErrEndpointNotFound
		}
		return nil, fmt.Errorf("get webhook: %w", err)
	}
	rows, err := repo.ListWebhookDeliveries(ctx, gen.ListWebhookDeliveriesParams{
		WorkspaceID: workspaceID, EndpointID: endpointID, AfterCreatedAt: afterAt, AfterID: afterID, PageSize: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	out := make([]model.Delivery, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.Delivery{
			ID: r.ID, EventID: r.EventID, EventType: r.EventType, Payload: r.Payload, Status: r.Status,
			Attempts: r.Attempts, ResponseStatus: r.ResponseStatus, ResponseMs: r.ResponseMs, Error: r.Error,
			CreatedAt: r.CreatedAt, DeliveredAt: r.DeliveredAt,
		})
	}
	return out, nil
}

// Redeliver sends a finished delivery again, with a fresh retry budget.
func (s *Service) Redeliver(ctx context.Context, workspaceID, endpointID, deliveryID uuid.UUID) error {
	return database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		n, err := repository.NewRepository(tx).ResetDeliveryForRetry(ctx, gen.ResetDeliveryForRetryParams{
			WorkspaceID: workspaceID, EndpointID: endpointID, ID: deliveryID,
		})
		if err != nil {
			return fmt.Errorf("reset delivery: %w", err)
		}
		if n == 0 {
			return model.ErrDeliveryNotFound
		}
		return s.enqueue(ctx, tx, workspaceID, deliveryID)
	})
}

// Publish records an event for every enabled endpoint subscribed to it and queues the
// deliveries, in tx: the event goes out exactly when the caller's change commits.
// Other features call it through a small interface wired in internal/app.
func (s *Service) Publish(ctx context.Context, tx pgx.Tx, workspaceID uuid.UUID, eventType string, data any) error {
	repo := repository.NewRepository(tx)
	endpoints, err := repo.ListSubscribedEndpoints(ctx, gen.ListSubscribedEndpointsParams{WorkspaceID: workspaceID, EventType: eventType})
	if err != nil {
		return fmt.Errorf("list subscribed webhooks: %w", err)
	}
	if len(endpoints) == 0 {
		return nil
	}
	ev, payload, err := s.newEvent(workspaceID, eventType, data)
	if err != nil {
		return err
	}
	for _, endpointID := range endpoints {
		if _, err := s.insertDelivery(ctx, tx, workspaceID, endpointID, ev, payload); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) newEvent(workspaceID uuid.UUID, eventType string, data any) (model.Event, []byte, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return model.Event{}, nil, fmt.Errorf("marshal event data: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Event{}, nil, fmt.Errorf("new id: %w", err)
	}
	ev := model.Event{ID: id, Type: eventType, CreatedAt: s.now(), WorkspaceID: workspaceID, Data: raw}
	payload, err := json.Marshal(ev)
	if err != nil {
		return model.Event{}, nil, fmt.Errorf("marshal event: %w", err)
	}
	return ev, payload, nil
}

func (s *Service) insertDelivery(ctx context.Context, tx pgx.Tx, workspaceID, endpointID uuid.UUID, ev model.Event, payload []byte) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("new id: %w", err)
	}
	err = repository.NewRepository(tx).CreateWebhookDelivery(ctx, gen.CreateWebhookDeliveryParams{
		ID: id, WorkspaceID: workspaceID, EndpointID: endpointID, EventID: ev.ID, EventType: ev.Type, Payload: payload,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("create delivery: %w", err)
	}
	return id, s.enqueue(ctx, tx, workspaceID, id)
}

func (s *Service) enqueue(ctx context.Context, tx pgx.Tx, workspaceID, deliveryID uuid.UUID) error {
	if _, err := s.queue.InsertTx(ctx, tx, model.DeliverArgs{WorkspaceID: workspaceID, DeliveryID: deliveryID}, nil); err != nil {
		return fmt.Errorf("enqueue delivery: %w", err)
	}
	return nil
}

// SendTest delivers a ping to the endpoint now, whether or not it's subscribed or on,
// and returns the logged delivery.
func (s *Service) SendTest(ctx context.Context, workspaceID, endpointID uuid.UUID) (model.Delivery, error) {
	repo := repository.NewRepository(s.pool)
	if _, err := repo.GetWebhookEndpoint(ctx, gen.GetWebhookEndpointParams{WorkspaceID: workspaceID, ID: endpointID}); err != nil {
		if repository.IsNoRows(err) {
			return model.Delivery{}, model.ErrEndpointNotFound
		}
		return model.Delivery{}, fmt.Errorf("get webhook: %w", err)
	}
	ev, payload, err := s.newEvent(workspaceID, model.EventPing, map[string]string{"message": "Test event from Relay"})
	if err != nil {
		return model.Delivery{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return model.Delivery{}, fmt.Errorf("new id: %w", err)
	}
	if err := repo.CreateWebhookDelivery(ctx, gen.CreateWebhookDeliveryParams{
		ID: id, WorkspaceID: workspaceID, EndpointID: endpointID, EventID: ev.ID, EventType: ev.Type, Payload: payload,
	}); err != nil {
		return model.Delivery{}, fmt.Errorf("create delivery: %w", err)
	}
	// A test isn't retried and doesn't count toward turning the endpoint off.
	if err := s.deliver(ctx, workspaceID, id, true, false); err != nil && s.log != nil {
		s.log.InfoContext(ctx, "test webhook failed", "endpoint_id", endpointID, "err", err)
	}
	list, err := s.Deliveries(ctx, workspaceID, endpointID, nil, nil, 50)
	if err != nil {
		return model.Delivery{}, err
	}
	for _, d := range list {
		if d.ID == id {
			return d, nil
		}
	}
	return model.Delivery{}, model.ErrDeliveryNotFound
}

// CleanupDeliveries deletes delivery log rows past the retention period.
func (s *Service) CleanupDeliveries(ctx context.Context) (int64, error) {
	n, err := repository.NewRepository(s.pool).DeleteOldDeliveries(ctx, s.now().Add(-model.DeliveryRetention))
	if err != nil {
		return 0, fmt.Errorf("delete old deliveries: %w", err)
	}
	return n, nil
}
