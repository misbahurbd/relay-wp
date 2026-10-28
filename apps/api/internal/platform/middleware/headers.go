package middleware

import "net/http"

// SecurityHeaders sets the headers every API response should carry: no
// framing (clickjacking), no MIME sniffing, and no Referer, since some URLs
// here carry one-time values (OAuth callbacks, redirects).
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
