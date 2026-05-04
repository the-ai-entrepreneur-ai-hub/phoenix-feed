// Package api contains the read-only HTTP API handlers.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

// Store is the database surface needed by the API handlers.
type Store interface {
	ListActiveIncidents(context.Context, store.ActiveIncidentFilter) (store.ActiveIncidentsResult, error)
}

// Config carries API-specific runtime behavior.
type Config struct {
	DefaultParserVersion string
}

// Router returns a standalone v1 API router. cmd/api wraps this with process
// middleware and server lifecycle.
func Router(st Store, cfg Config, log *slog.Logger) http.Handler {
	r := chi.NewRouter()
	RegisterRoutes(r, st, cfg, log)
	return r
}

// RegisterRoutes adds API routes to an existing chi router.
func RegisterRoutes(r chi.Router, st Store, cfg Config, log *slog.Logger) {
	r.Get("/v1/incidents/active", activeIncidentsHandler(st, cfg, log))
}
