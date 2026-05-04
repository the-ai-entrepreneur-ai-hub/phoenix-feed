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

func runSweep(_ context.Context, _ *store.Store, _ config.Config, log *slog.Logger) {
	// TODO:
	//   - UPDATE incidents SET raw=NULL, raw_dropped_at=NOW()
	//     WHERE raw IS NOT NULL AND last_seen_at < NOW() - $RAW_RETENTION;
	//   - Archive cleared incidents older than 30 days to a cold partition
	//     (or just delete; design decision deferred).
	//   - Vacuum / analyze if necessary.
	log.Info("janitor sweep (todo)")
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
