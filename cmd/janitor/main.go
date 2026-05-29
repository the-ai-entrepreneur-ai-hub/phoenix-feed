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

const (
	rawSweepInterval = 6 * time.Hour
	sdrSweepInterval = time.Minute
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

	log.Info("janitor starting",
		"raw_retention", cfg.RawRetention,
		"raw_interval", rawSweepInterval,
		"sdr_active_window", cfg.SDRActiveWindow,
		"sdr_interval", sdrSweepInterval,
	)

	runSDRSweep(ctx, st, cfg, log)

	rawTick := time.NewTicker(rawSweepInterval)
	defer rawTick.Stop()
	sdrTick := time.NewTicker(sdrSweepInterval)
	defer sdrTick.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("janitor shutting down")
			return
		case <-rawTick.C:
			runRawSweep(ctx, st, cfg, log)
		case <-sdrTick.C:
			runSDRSweep(ctx, st, cfg, log)
		}
	}
}

func runRawSweep(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) {
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

func runSDRSweep(ctx context.Context, st *store.Store, cfg config.Config, log *slog.Logger) {
	sweepCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	now := time.Now().UTC()
	cleared, err := st.ClearStaleSDRAudioIncidents(sweepCtx, now, cfg.SDRActiveWindow)
	if err != nil {
		log.Error("clear stale sdr incidents", "err", err)
		return
	}
	log.Info("janitor sdr sweep", "sdr_rows_cleared", cleared, "sdr_active_window", cfg.SDRActiveWindow)
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
