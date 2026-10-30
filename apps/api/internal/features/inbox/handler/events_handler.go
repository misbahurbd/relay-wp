package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/misbahurbd/relay-wp/apps/api/internal/features/inbox/dto"
	"github.com/misbahurbd/relay-wp/apps/api/internal/platform/logger"
)

// Live-update stream timings (AD-8).
const (
	heartbeatEvery = 20 * time.Second
	streamLifetime = 15 * time.Minute
	retryMillis    = 3000
)

// events streams live-update signals as Server-Sent Events (AD-8, ADR 0009).
// Each frame is "event: <kind>" and "data: <json>"; a comment every 20 s keeps
// proxies from closing the stream. It ends after 15 minutes so the browser
// reconnects with fresh access, and drops a client that falls behind.
func (h *Handler) events(ctx context.Context, _ *dto.InboxWorkspaceInput) (*huma.StreamResponse, error) {
	a := actor(ctx)
	ch, stop, err := h.svc.Subscribe(a)
	if err != nil {
		return nil, err
	}
	return &huma.StreamResponse{Body: func(hctx huma.Context) {
		defer stop()
		w, _ := hctx.BodyWriter().(http.ResponseWriter)
		var rc *http.ResponseController
		if w != nil {
			rc = http.NewResponseController(w)
			// This one route outlives the server's write timeout.
			_ = rc.SetWriteDeadline(time.Time{})
		}
		flush := func() {
			if rc != nil {
				_ = rc.Flush()
			}
		}
		hctx.SetHeader("Content-Type", "text/event-stream")
		hctx.SetHeader("Cache-Control", "no-store")
		hctx.SetHeader("Connection", "keep-alive")
		hctx.SetHeader("X-Accel-Buffering", "no")
		hctx.SetStatus(http.StatusOK)
		out := hctx.BodyWriter()
		if _, err := fmt.Fprintf(out, "retry: %d\n: connected\n\n", retryMillis); err != nil {
			return
		}
		flush()

		heartbeat := time.NewTicker(heartbeatEvery)
		defer heartbeat.Stop()
		lifetime := time.NewTimer(streamLifetime)
		defer lifetime.Stop()
		done := hctx.Context().Done()
		for {
			select {
			case <-done:
				return
			case <-lifetime.C:
				return
			case <-heartbeat.C:
				if _, err := io.WriteString(out, ": ping\n\n"); err != nil {
					return
				}
			case ev, ok := <-ch:
				if !ok {
					return // fell behind; the browser reconnects and refetches
				}
				data, err := json.Marshal(ev)
				if err != nil {
					logger.FromContext(ctx).Error("inbox event encode failed", "err", err)
					continue
				}
				if _, err := fmt.Fprintf(out, "event: %s\ndata: %s\n\n", ev.Kind, data); err != nil {
					return
				}
			}
			flush()
		}
	}}, nil
}
