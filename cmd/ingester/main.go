// Command ingester polls one upstream source on a configured cadence and
// hands the result to the lifecycle manager. One process per source.
package main

import (
	"context"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/lifecycle"
	"github.com/abusedmindset/phoenix-feed/internal/source"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	log := newLogger(cfg.LogLevel)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("store", err)
	}
	defer st.Close()

	src := pickSource()
	mgr := lifecycle.New(st, cfg.ClearAfterMisses, log)

	log.Info("ingester starting",
		"source", src.Name(),
		"parser_version", src.ParserVersion(),
		"interval", cfg.PollInterval,
		"jitter", cfg.PollJitter,
		"clear_after_misses", cfg.ClearAfterMisses,
	)

	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			log.Info("ingester shutting down")
			return
		default:
		}

		// Initial wait so we never poll back-to-back at startup.
		wait := lifecycle.Jitter(cfg.PollInterval, cfg.PollJitter, rnd.Float64) + backoff
		log.Debug("sleep", "duration", wait)
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return
		}

		pollCtx, cancelPoll := context.WithTimeout(ctx, 30*time.Second)
		res := src.Poll(pollCtx)
		cancelPoll()

		if applyErr := mgr.Apply(ctx, res); applyErr != nil {
			log.Error("lifecycle apply", "err", applyErr)
		}

		// Exponential backoff on transport / 5xx; reset on success.
		switch {
		case res.Success():
			backoff = 0
		case res.StatusCode == 429 || res.StatusCode == 403:
			log.Warn("rate-limited or forbidden, escalating backoff", "status", res.StatusCode)
			backoff = capBackoff(backoff*2 + 30*time.Second)
		case !res.Success():
			backoff = capBackoff(backoff*2 + 5*time.Second)
		}
	}
}

func pickSource() source.Source {
	// Today: only Phoenix Fire MapServer. When scanner+Whisper lands, this
	// becomes a switch on $SOURCE_NAME and returns one of N implementations.
	return phxfire.New()
}

func capBackoff(d time.Duration) time.Duration {
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
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
