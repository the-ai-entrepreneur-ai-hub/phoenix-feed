// Command janitor handles retention. Drops raw JSONB after the configured
// age, archives old cleared incidents to cold storage. Skeleton.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const sweepInterval = 6 * time.Hour

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

	log.Info("janitor starting", "raw_retention", cfg.RawRetention, "interval", sweepInterval)

	tick := time.NewTicker(sweepInterval)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("janitor shutting down")
			return
		case <-tick.C:
			runSweep(ctx, st, cfg, log)
		}
	}
}

func runSweep(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) {
	sweepCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	now := time.Now().UTC()
	dropped, err := st.DropRawOlderThan(sweepCtx, now, cfg.RawRetention)
	if err != nil {
		log.Error("drop old raw json", "err", err)
		return
	}
	if err := st.VacuumAnalyzeIncidents(sweepCtx); err != nil {
		log.Error("vacuum incidents", "err", err)
		return
	}
	log.Info("janitor sweep", "raw_rows_dropped", dropped, "raw_retention", cfg.RawRetention)
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
