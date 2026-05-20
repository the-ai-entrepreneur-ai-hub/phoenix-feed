// Command api serves the read-only REST/JSON cache to clients. Skeleton:
// only /v1/health is wired. Endpoints land here as the spec firms up.
package main

import (
	"context"
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

	api.RegisterRoutes(r, st, api.Config{
		DefaultParserVersion: phxfire.ParserVersion,
		AdminToken:           cfg.AdminToken,
		AllowedOrigins:       cfg.AllowedOrigins,
		FireStationsURL:      cfg.FireStationsURL,
		PaidTierEnabled:      cfg.PaidTierEnabled,
		Sources:              []string{phxfire.SourceName},
		StaleAfter:           10 * time.Minute,
	}, log)

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

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
