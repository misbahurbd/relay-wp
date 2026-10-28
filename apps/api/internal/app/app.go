// Package app is the composition root: it builds dependencies and wires features together.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// App holds the HTTP router, the huma API and everything wired into them.
type App struct {
	addr   string
	router chi.Router
	api    huma.API
}

// New builds the App and registers all routes.
func New() *App {
	router := chi.NewRouter()
	// Client IP handling (trusted proxies only) is added with the Phase 1 middleware.
	router.Use(middleware.RequestID, middleware.Recoverer)

	cfg := huma.DefaultConfig("Relay API", "0.1.0")
	api := humachi.New(router, cfg)

	a := &App{
		addr:   envOr("HTTP_ADDR", ":8080"),
		router: router,
		api:    api,
	}
	a.registerRoutes()
	return a
}

// OpenAPI returns the generated OpenAPI document.
func (a *App) OpenAPI() *huma.OpenAPI {
	return a.api.OpenAPI()
}

// Run serves HTTP until ctx is cancelled, then shuts down gracefully.
func (a *App) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              a.addr,
		Handler:           a.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("http server listening", "addr", a.addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}
	return nil
}

// envOr is a stopgap until platform/config lands in Phase 1.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
