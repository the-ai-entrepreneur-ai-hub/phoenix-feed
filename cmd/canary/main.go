// Command canary runs hourly schema and shape checks against each upstream
// source and writes results to contract_canary. Pages on drift before users
// notice. Skeleton: outer loop wired; per-source check logic is TODO.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/canary"
	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const checkInterval = 1 * time.Hour

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

	log.Info("canary starting", "interval", checkInterval)

	tick := time.NewTicker(checkInterval)
	defer tick.Stop()

	// Run once at startup so we don't wait an hour for the first signal.
	runChecks(ctx, st, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("canary shutting down")
			return
		case <-tick.C:
			runChecks(ctx, st, log)
		}
	}
}

func runChecks(ctx context.Context, st *store.Store, log *slog.Logger) {
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	result, err := canary.NewPhoenixFire(st, log).Run(pollCtx)
	if err != nil {
		log.Error("canary run", "err", err)
		return
	}
	log.Info("canary check", "source", result.Source, "passed", result.Passed, "feature_count", result.FeatureCount)
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
