package realtime

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/database/dbtest"
)

func quietHub() *Hub { return New(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), InboxChannel) }

func ptr(id uuid.UUID) *uuid.UUID { return &id }

func TestDispatchByWorkspaceAndFilter(t *testing.T) {
	h := quietHub()
	ws, other := uuid.New(), uuid.New()
	team := uuid.New()

	all, stopAll := h.Subscribe(ws, nil)
	defer stopAll()
	teamOnly, stopTeam := h.Subscribe(ws, func(e Event) bool { return e.TeamID != nil && *e.TeamID == team })
	defer stopTeam()
	elsewhere, stopElsewhere := h.Subscribe(other, nil)
	defer stopElsewhere()

	h.dispatch(Event{WorkspaceID: ws, Kind: "message", TeamID: ptr(team)})
	h.dispatch(Event{WorkspaceID: ws, Kind: "status"})

	if got := drain(all); len(got) != 2 {
		t.Errorf("unfiltered subscriber got %d events, want 2", len(got))
	}
	if got := drain(teamOnly); len(got) != 1 || got[0].Kind != "message" {
		t.Errorf("team subscriber got %+v", got)
	}
	if got := drain(elsewhere); len(got) != 0 {
		t.Errorf("other workspace got %+v", got)
	}
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	h := quietHub()
	ws := uuid.New()
	slow, stopSlow := h.Subscribe(ws, nil)
	defer stopSlow()
	fast, stopFast := h.Subscribe(ws, nil)
	defer stopFast()

	for range SubscriberBuffer {
		h.dispatch(Event{WorkspaceID: ws, Kind: "message"})
		<-fast
	}
	h.dispatch(Event{WorkspaceID: ws, Kind: "message"}) // one past the slow buffer

	n := 0
	for range slow { // closed once dropped, after the buffered events
		n++
	}
	if n != SubscriberBuffer {
		t.Errorf("slow subscriber drained %d events, want %d", n, SubscriberBuffer)
	}
	if ev, ok := <-fast; !ok || ev.Kind != "message" {
		t.Error("fast subscriber should still get events")
	}
	if h.Subscribers() != 1 {
		t.Errorf("subscribers = %d, want 1", h.Subscribers())
	}
}

func TestUnsubscribeClosesOnce(t *testing.T) {
	h := quietHub()
	ws := uuid.New()
	ch, stop := h.Subscribe(ws, nil)
	stop()
	stop()
	if _, ok := <-ch; ok {
		t.Error("channel should be closed")
	}
	h.dispatch(Event{WorkspaceID: ws}) // no subscriber, no panic
	if h.Subscribers() != 0 {
		t.Errorf("subscribers = %d", h.Subscribers())
	}
}

func TestListenDeliversCommittedNotifications(t *testing.T) {
	pool := dbtest.New(t)
	h := New(pool, slog.New(slog.NewTextHandler(io.Discard, nil)), InboxChannel)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { _ = h.Run(ctx); close(done) }()
	defer func() { cancel(); <-done }()

	ws := uuid.New()
	ch, stop := h.Subscribe(ws, nil)
	defer stop()
	conv := uuid.New()

	// The listener connects in the background; notify until one arrives.
	deadline := time.After(10 * time.Second)
	for {
		// A rolled-back notification is never delivered.
		tx, err := pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := Notify(t.Context(), tx, InboxChannel, Event{WorkspaceID: ws, Kind: "rolled_back"}); err != nil {
			t.Fatal(err)
		}
		_ = tx.Rollback(t.Context())
		if err := pgx.BeginFunc(t.Context(), pool, func(tx pgx.Tx) error {
			return Notify(t.Context(), tx, InboxChannel, Event{WorkspaceID: ws, Kind: "message", ConversationID: &conv})
		}); err != nil {
			t.Fatal(err)
		}
		select {
		case ev := <-ch:
			if ev.Kind != "message" || ev.ConversationID == nil || *ev.ConversationID != conv {
				t.Fatalf("got %+v", ev)
			}
			return
		case <-time.After(200 * time.Millisecond):
		case <-deadline:
			t.Fatal("no notification delivered")
		}
	}
}

func drain(ch <-chan Event) []Event {
	var out []Event
	for {
		select {
		case ev := <-ch:
			out = append(out, ev)
		default:
			return out
		}
	}
}
