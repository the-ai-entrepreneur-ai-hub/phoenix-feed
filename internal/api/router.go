// Package api contains the read-only HTTP API handlers.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

// Store is the database surface needed by the API handlers.
type Store interface {
	GetIncidentDetail(context.Context, string, string) (store.IncidentDetailResult, error)
	ListActiveIncidents(context.Context, store.ActiveIncidentFilter) (store.ActiveIncidentsResult, error)
	LookupAPIKey(context.Context, string) (store.APIKey, error)
	Ping(context.Context) error
	SourceHealth(context.Context, []string) ([]store.SourceHealth, error)
}

// Config carries API-specific runtime behavior.
type Config struct {
	DefaultParserVersion string
	AllowedOrigins       []string
	PaidTierEnabled      bool
	Sources              []string
	StaleAfter           time.Duration
	RateLimiter          *ratelimit.Limiter
	Now                  func() time.Time
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
	r.Use(corsMiddleware(cfg.AllowedOrigins))
	r.Use(authMiddleware(st, log))
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	limiter := cfg.RateLimiter
	if limiter == nil {
		limiter = ratelimit.NewDefault()
	}
	cfg.RateLimiter = limiter

	r.With(rateLimitMiddleware(limiter, ratelimit.ScopeIncidentRead)).Get("/v1/incidents/active", activeIncidentsHandler(st, cfg, log))
	r.With(rateLimitMiddleware(limiter, ratelimit.ScopeManualRefresh)).Post("/v1/incidents/refresh", refreshIncidentsHandler(st, cfg, log))
	r.Get("/v1/incidents/history", historyPlaceholderHandler(cfg))
	r.With(rateLimitMiddleware(limiter, ratelimit.ScopeIncidentRead)).Get("/v1/incidents/{source}/{incident_id}", incidentDetailHandler(st, cfg, log))
	r.Get("/v1/health", healthHandler(st, cfg, log))
}
