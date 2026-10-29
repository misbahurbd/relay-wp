package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS allows the configured browser origins to call the API with credentials (cookies).
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-Id", AccessVersionHeader},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
