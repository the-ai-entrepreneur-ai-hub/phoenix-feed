// Command dispatch-parser promotes high-confidence SDR dispatch transcripts
// into incident rows that the existing active feed can serve.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/config"
	dispatchparser "github.com/abusedmindset/phoenix-feed/internal/dispatch/parser"
	"github.com/abusedmindset/phoenix-feed/internal/geocode"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	if cfg.MapboxToken == "" {
		fatal("config", errors.New("MAPBOX_TOKEN is required for dispatch-parser; provision a server-side Mapbox token before deploy"))
	}
	log := newLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("pgx pool", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("pgx ping", err)
	}

	mapbox, err := geocode.NewMapboxClient(cfg.MapboxToken)
	if err != nil {
		fatal("mapbox", err)
	}
	cached := geocode.NewCachedGeocoder(geocode.NewPostgresCache(pool), mapbox, nil)
	worker := dispatchparser.NewWorker(pool, cached, log, dispatchparser.WorkerOptions{MaxAge: cfg.DispatchMaxAge})

	log.Info("dispatch parser starting", "interval", 5*time.Second, "parser_version", dispatchparser.ParserVersion, "dispatch_max_age", cfg.DispatchMaxAge)
	worker.Run(ctx, 5*time.Second)
	log.Info("dispatch parser shutting down")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
