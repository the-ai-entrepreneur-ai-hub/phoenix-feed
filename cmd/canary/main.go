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

	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
)

const checkInterval = 1 * time.Hour

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	log.Info("canary starting", "interval", checkInterval)

	tick := time.NewTicker(checkInterval)
	defer tick.Stop()

	// Run once at startup so we don't wait an hour for the first signal.
	runChecks(ctx, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("canary shutting down")
			return
		case <-tick.C:
			runChecks(ctx, log)
		}
	}
}

func runChecks(ctx context.Context, log *slog.Logger) {
	// TODO(architecture.md §7):
	//   - reachable, HTTP 200
	//   - all expected fields present
	//   - outSR=4326 honored (geometry plausibility check)
	//   - feature_count plausible vs 7-day baseline
	//   - Date parses as recent epoch ms
	//   - sample feature parses cleanly through prod parser
	// On drift: write contract_canary row with passed=false and structured diff,
	// emit alert (PagerDuty / OpsGenie / webhook — TBD).

	c := phxfire.New()
	pollCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	res := c.Poll(pollCtx)

	log.Info("canary check",
		"source", res.Source,
		"success", res.Success(),
		"status", res.StatusCode,
		"feature_count", len(res.Incidents),
		"latency_ms", res.LatencyMS,
	)
}
