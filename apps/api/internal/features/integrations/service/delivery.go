package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/misbahurbd/relay-wp/apps/api/db/gen"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/model"
	"github.com/misbahurbd/relay-wp/apps/api/internal/features/integrations/repository"
)

// Headers on every delivery. The signature scheme matches Stripe's, so existing
// verifier code works: HMAC-SHA256 of "<t>.<body>" with the endpoint's secret.
const (
	HeaderSignature = "Relay-Signature"
	HeaderEvent     = "Relay-Event"
	HeaderDelivery  = "Relay-Delivery"
	userAgent       = "Relay-Webhooks/1.0"
	deliverTimeout  = 10 * time.Second
	maxErrorLen     = 300
)

var errBlockedAddress = errors.New("address is private or reserved")

// errDeliveryFailed is returned to River so it retries; the cause is already logged on the delivery.
var errDeliveryFailed = errors.New("webhook delivery failed")

// Sign returns the Relay-Signature header value for body sent at t.
func Sign(secret string, t time.Time, body []byte) string {
	ts := strconv.FormatInt(t.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts + "."))
	mac.Write(body)
	return "t=" + ts + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

// Deliver sends one queued delivery. lastAttempt marks a failure as final, which
// counts toward turning the endpoint off. The error makes River retry.
func (s *Service) Deliver(ctx context.Context, workspaceID, deliveryID uuid.UUID, lastAttempt bool) error {
	return s.deliver(ctx, workspaceID, deliveryID, lastAttempt, true)
}

func (s *Service) deliver(ctx context.Context, workspaceID, deliveryID uuid.UUID, final, countFailure bool) error {
	repo := repository.NewRepository(s.pool)
	d, err := repo.GetDeliveryForSend(ctx, gen.GetDeliveryForSendParams{WorkspaceID: workspaceID, ID: deliveryID})
	if repository.IsNoRows(err) {
		return nil // the endpoint was deleted
	}
	if err != nil {
		return fmt.Errorf("get delivery: %w", err)
	}
	if d.Status != model.DeliveryPending {
		return nil
	}
	if !d.Enabled && countFailure {
		msg := "Endpoint is turned off"
		return s.recordAttempt(ctx, repo, d, model.DeliveryFailed, nil, nil, &msg)
	}
	secret, err := s.box.open(d.Secret)
	if err != nil {
		return err
	}

	start := time.Now()
	code, sendErr := s.post(ctx, d, string(secret))
	ms := int32(min(time.Since(start).Milliseconds(), 1<<31-1)) //nolint:gosec // clamped
	var status *int32
	if code > 0 {
		c := int32(code) //nolint:gosec // HTTP status codes fit
		status = &c
	}
	if sendErr == nil && code >= 200 && code < 300 {
		if err := s.recordAttempt(ctx, repo, d, model.DeliverySucceeded, status, &ms, nil); err != nil {
			return err
		}
		if countFailure {
			if err := repo.RecordEndpointSuccess(ctx, gen.RecordEndpointSuccessParams{WorkspaceID: workspaceID, ID: d.EndpointID}); err != nil {
				return fmt.Errorf("record webhook success: %w", err)
			}
		}
		return nil
	}

	reason := describeFailure(code, sendErr)
	next := model.DeliveryPending
	if final {
		next = model.DeliveryFailed
	}
	if err := s.recordAttempt(ctx, repo, d, next, status, &ms, &reason); err != nil {
		return err
	}
	if final && countFailure {
		on, err := repo.RecordEndpointFailure(ctx, gen.RecordEndpointFailureParams{
			MaxFailures: model.MaxConsecutiveFailures, WorkspaceID: workspaceID, ID: d.EndpointID,
			Reason: fmt.Sprintf("Turned off after %d deliveries in a row failed", model.MaxConsecutiveFailures),
		})
		if err != nil {
			return fmt.Errorf("record webhook failure: %w", err)
		}
		if !on && s.log != nil {
			s.log.WarnContext(ctx, "webhook endpoint turned off", "workspace_id", workspaceID, "endpoint_id", d.EndpointID)
		}
	}
	if s.log != nil {
		s.log.InfoContext(ctx, "webhook delivery failed", "endpoint_id", d.EndpointID, "status", code, "ms", ms, "final", final)
	}
	return errDeliveryFailed
}

func (s *Service) recordAttempt(ctx context.Context, repo repository.Repository, d gen.GetDeliveryForSendRow, status string, code, ms *int32, reason *string) error {
	err := repo.RecordDeliveryAttempt(ctx, gen.RecordDeliveryAttemptParams{
		Status: status, ResponseStatus: code, ResponseMs: ms, Error: reason, WorkspaceID: d.WorkspaceID, ID: d.ID,
	})
	if err != nil {
		return fmt.Errorf("record delivery: %w", err)
	}
	return nil
}

func (s *Service) post(ctx context.Context, d gen.GetDeliveryForSendRow, secret string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, deliverTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.Url, bytes.NewReader(d.Payload))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set(HeaderEvent, d.EventType)
	req.Header.Set(HeaderDelivery, d.ID.String())
	req.Header.Set(HeaderSignature, Sign(secret, s.now(), d.Payload))
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	return resp.StatusCode, nil
}

func describeFailure(code int, err error) string {
	var msg string
	switch {
	case errors.Is(err, errBlockedAddress):
		msg = "The URL points to a private or reserved address"
	case errors.Is(err, context.DeadlineExceeded):
		msg = "No response within 10 seconds"
	case err != nil:
		msg = err.Error()
	case code >= 300 && code < 400:
		msg = fmt.Sprintf("Got a %d redirect; redirects aren't followed", code)
	default:
		msg = fmt.Sprintf("Got HTTP %d", code)
	}
	if len(msg) > maxErrorLen {
		msg = msg[:maxErrorLen]
	}
	return msg
}

// newSafeClient builds the client for URLs people enter: no proxy, no redirects,
// and (unless allowPrivate) no connections to private or reserved addresses,
// checked after DNS so a name can't resolve its way around the rule.
func newSafeClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	if !allowPrivate {
		dialer.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("dial address: %w", err)
			}
			if isBlockedHost(host) {
				return errBlockedAddress
			}
			return nil
		}
	}
	return &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         dialer.DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// cgnat is 100.64.0.0/10, shared address space some clouds use internally.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// isBlockedHost reports whether host is an IP that webhooks and store checks must not reach.
// Names pass here; the dialer checks the address they resolve to.
func isBlockedHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() || cgnat.Contains(ip) ||
		(ip.Is4() && ip.As4()[0] == 0)
}
