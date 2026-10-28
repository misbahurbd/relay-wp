// Package middleware holds HTTP middleware shared by every route.
package middleware

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIP returns the client address resolved by ClientIPResolver.
func ClientIP(ctx context.Context) string {
	ip, _ := ctx.Value(clientIPKey{}).(string)
	return ip
}

// ClientIPResolver finds the real client IP. X-Forwarded-For is only believed
// when the direct peer is a trusted proxy; otherwise anyone could spoof it.
type ClientIPResolver struct {
	trusted []netip.Prefix
}

// NewClientIPResolver parses trusted proxy CIDRs (e.g. "10.0.0.0/8").
func NewClientIPResolver(cidrs []string) (*ClientIPResolver, error) {
	r := &ClientIPResolver{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		p, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("invalid TRUSTED_PROXIES entry %q: %w", c, err)
		}
		r.trusted = append(r.trusted, p.Masked())
	}
	return r, nil
}

// Middleware stores the resolved IP in the request context.
func (r *ClientIPResolver) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), clientIPKey{}, r.resolve(req))
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func (r *ClientIPResolver) resolve(req *http.Request) string {
	peer := remoteIP(req.RemoteAddr)
	if !peer.IsValid() {
		return req.RemoteAddr
	}
	if !r.isTrusted(peer) {
		return peer.String()
	}

	// Walk X-Forwarded-For right to left; the first untrusted hop is the client.
	hops := strings.Split(strings.Join(req.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			break
		}
		ip = ip.Unmap()
		if !r.isTrusted(ip) {
			return ip.String()
		}
	}
	return peer.String()
}

func (r *ClientIPResolver) isTrusted(ip netip.Addr) bool {
	for _, p := range r.trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) netip.Addr {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}
