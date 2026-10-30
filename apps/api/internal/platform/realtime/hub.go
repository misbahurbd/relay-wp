package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubscriberBuffer is how many events a subscriber may fall behind before it is dropped.
const SubscriberBuffer = 64

// Reconnect backoff for the LISTEN connection.
const (
	minBackoff = 500 * time.Millisecond
	maxBackoff = 30 * time.Second
)

// Hub listens on one channel and fans its events out to subscribers by workspace.
type Hub struct {
	pool    *pgxpool.Pool
	log     *slog.Logger
	channel string

	mu   sync.RWMutex
	subs map[uuid.UUID]map[*subscriber]struct{}
}

type subscriber struct {
	ch     chan Event
	filter func(Event) bool
	closed bool
}

// New builds a hub for channel on pool, which must be the direct (unpooled)
// pool: LISTEN does not survive a transaction-mode pooler. Run starts it.
func New(pool *pgxpool.Pool, log *slog.Logger, channel string) *Hub {
	if log == nil {
		log = slog.Default()
	}
	return &Hub{pool: pool, log: log, channel: channel, subs: map[uuid.UUID]map[*subscriber]struct{}{}}
}

// Subscribe returns the events for workspaceID that filter accepts (nil
// accepts all), and a function that ends the subscription. The channel is
// closed when the subscription ends, including when the subscriber falls more
// than SubscriberBuffer events behind; the client then reconnects and refetches.
func (h *Hub) Subscribe(workspaceID uuid.UUID, filter func(Event) bool) (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, SubscriberBuffer), filter: filter}
	h.mu.Lock()
	if h.subs[workspaceID] == nil {
		h.subs[workspaceID] = map[*subscriber]struct{}{}
	}
	h.subs[workspaceID][s] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return s.ch, func() { once.Do(func() { h.remove(workspaceID, s) }) }
}

func (h *Hub) remove(workspaceID uuid.UUID, s *subscriber) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.removeLocked(workspaceID, s)
}

func (h *Hub) removeLocked(workspaceID uuid.UUID, s *subscriber) {
	if set := h.subs[workspaceID]; set != nil {
		delete(set, s)
		if len(set) == 0 {
			delete(h.subs, workspaceID)
		}
	}
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

// Subscribers is how many subscriptions are open, for logs and metrics.
func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.subs {
		n += len(set)
	}
	return n
}

// dispatch hands ev to every subscriber of its workspace whose filter accepts it.
// A subscriber whose buffer is full is dropped rather than blocking the others.
func (h *Hub) dispatch(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs[ev.WorkspaceID] {
		if s.filter != nil && !s.filter(ev) {
			continue
		}
		select {
		case s.ch <- ev:
		default:
			h.log.Warn("realtime: dropping slow subscriber", "workspace_id", ev.WorkspaceID)
			h.removeLocked(ev.WorkspaceID, s)
		}
	}
}

// Run listens until ctx is cancelled, reconnecting with backoff when the
// connection drops. While it is down, clients fall back to polling.
func (h *Hub) Run(ctx context.Context) error {
	backoff := minBackoff
	for {
		start := time.Now()
		err := h.listen(ctx)
		if ctx.Err() != nil {
			return nil //nolint:nilerr // cancelled: a normal stop
		}
		if time.Since(start) > time.Minute {
			backoff = minBackoff // it was up for a while; start over
		}
		h.log.Warn("realtime: listener lost, reconnecting", "channel", h.channel, "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

// listen holds one connection, taken out of the pool, for as long as it works.
func (h *Hub) listen(ctx context.Context) error {
	pc, err := h.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	conn := pc.Hijack() // a LISTENing connection must not go back to the pool
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{h.channel}.Sanitize()); err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	h.log.Info("realtime: listening", "channel", h.channel)
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait: %w", err)
		}
		if n.Channel != h.channel {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil || ev.WorkspaceID == uuid.Nil {
			h.log.Warn("realtime: ignoring malformed event", "err", errors.Join(err, errMissingWorkspace(ev)))
			continue
		}
		h.dispatch(ev)
	}
}

func errMissingWorkspace(ev Event) error {
	if ev.WorkspaceID == uuid.Nil {
		return errors.New("event has no workspace")
	}
	return nil
}
