// Command api serves the read-only REST/JSON cache to clients. Skeleton:
// only /v1/health is wired. Endpoints land here as the spec firms up.
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/abusedmindset/phoenix-feed/internal/api"
	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("store", err)
	}
	defer st.Close()

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Get("/v1/health", healthHandler(st, log))
	api.RegisterRoutes(r, st, api.Config{DefaultParserVersion: phxfire.ParserVersion}, log)

	// TODO: /v1/incidents/{source}/{id}, /v1/incidents/history (paid),
	// /v1/geofences (paid).

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fatal("listen", err)
		}
	}()

	<-ctx.Done()
	log.Info("api shutting down")
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

func healthHandler(st *store.Store, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		dbOK := st.Ping(ctx) == nil
		lastSuccess, _ := st.LatestSuccessAt(ctx, phxfire.SourceName)

		body := map[string]any{
			"ok":           dbOK,
			"db_reachable": dbOK,
			"sources": map[string]any{
				phxfire.SourceName: map[string]any{
					"last_success_at":       lastSuccess.Format(time.RFC3339),
					"seconds_since_success": int(time.Since(lastSuccess).Seconds()),
				},
			},
			"server_time": time.Now().UTC().Format(time.RFC3339),
		}
		status := http.StatusOK
		if !dbOK {
			status = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
