// Package config loads runtime configuration from environment variables.
// Keep it boring: no YAML, no flags, no precedence rules. Twelve-factor.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL         string        // postgres://...
	AdminToken          string        // bearer token for private admin endpoints
	PollInterval        time.Duration // ingester base cadence
	PollJitter          time.Duration // ± jitter applied each cycle
	HTTPAddr            string        // api bind address
	FireStationsURL     string        // optional upstream override for station overlay proxy
	MapboxToken         string        // server-side Mapbox geocoding token for SDR parser
	ClearAfterMisses    int           // consecutive successful polls absent before clearing
	SourceDegradedAfter time.Duration // source status: delayed authoritative success
	SourceDownAfter     time.Duration // source status: unavailable authoritative success
	SourceDownFailures  int           // consecutive failed polls before source is down
	FrozenRepeatCount   int           // identical non-empty polls before degradation
	FrozenDownAfter     time.Duration // identical non-empty duration before source is down
	SuddenCollapsePct   int           // feature-count drop requiring count confirmation
	RawRetention        time.Duration // janitor: drop raw JSONB after this age
	DispatchMaxAge      time.Duration // parser: reject SDR transcripts older than this age
	SDRActiveWindow     time.Duration // janitor: clear SDR incidents after this age
	LogLevel            string        // "debug" | "info" | "warn" | "error"
	PaidTierEnabled     bool          // gates paid placeholders; default false
	AllowedOrigins      []string      // exact browser origins allowed for CORS
}

const (
	DefaultDispatchMaxAge  = 2 * time.Hour
	DefaultSDRActiveWindow = 90 * time.Minute
)

func Load() (Config, error) {
	c := Config{
		DatabaseURL:         env("DATABASE_URL", "postgres://localhost:5432/phoenix_feed?sslmode=disable"),
		AdminToken:          env("ADMIN_TOKEN", ""),
		HTTPAddr:            env("HTTP_ADDR", ":8080"),
		FireStationsURL:     env("FIRE_STATIONS_URL", ""),
		MapboxToken:         env("MAPBOX_TOKEN", ""),
		LogLevel:            env("LOG_LEVEL", "info"),
		ClearAfterMisses:    envInt("CLEAR_AFTER_MISSES", 5),
		SourceDegradedAfter: envDuration("SOURCE_DEGRADED_AFTER", 3*time.Minute),
		SourceDownAfter:     envDuration("SOURCE_DOWN_AFTER", 10*time.Minute),
		SourceDownFailures:  envInt("SOURCE_DOWN_FAILURES", 3),
		FrozenRepeatCount:   envInt("FROZEN_REPEAT_COUNT", 3),
		FrozenDownAfter:     envDuration("FROZEN_DOWN_AFTER", 10*time.Minute),
		SuddenCollapsePct:   envInt("SUDDEN_COLLAPSE_PERCENT", 80),
		PollInterval:        envDuration("POLL_INTERVAL", 60*time.Second),
		PollJitter:          envDuration("POLL_JITTER", 10*time.Second),
		RawRetention:        envDuration("RAW_RETENTION", 30*24*time.Hour),
		DispatchMaxAge:      envDuration("DISPATCH_MAX_AGE", DefaultDispatchMaxAge),
		SDRActiveWindow:     envDuration("SDR_ACTIVE_WINDOW", DefaultSDRActiveWindow),
		PaidTierEnabled:     envBool("PAID_TIER_ENABLED", false),
		AllowedOrigins:      envList("ALLOWED_ORIGINS"),
	}

	if c.PollJitter >= c.PollInterval {
		return c, fmt.Errorf("POLL_JITTER (%s) must be less than POLL_INTERVAL (%s)", c.PollJitter, c.PollInterval)
	}
	if c.ClearAfterMisses < 1 {
		return c, fmt.Errorf("CLEAR_AFTER_MISSES must be >= 1")
	}
	if c.SourceDegradedAfter <= 0 || c.SourceDownAfter <= c.SourceDegradedAfter {
		return c, fmt.Errorf("source thresholds require 0 < SOURCE_DEGRADED_AFTER < SOURCE_DOWN_AFTER")
	}
	if c.SourceDownFailures < 1 {
		return c, fmt.Errorf("SOURCE_DOWN_FAILURES must be >= 1")
	}
	if c.FrozenRepeatCount < 2 || c.FrozenDownAfter <= 0 {
		return c, fmt.Errorf("frozen thresholds require FROZEN_REPEAT_COUNT >= 2 and FROZEN_DOWN_AFTER > 0")
	}
	if c.SuddenCollapsePct < 1 || c.SuddenCollapsePct > 100 {
		return c, fmt.Errorf("SUDDEN_COLLAPSE_PERCENT must be between 1 and 100")
	}
	if c.DispatchMaxAge <= 0 {
		return c, fmt.Errorf("DISPATCH_MAX_AGE must be positive")
	}
	if c.SDRActiveWindow <= 0 {
		return c, fmt.Errorf("SDR_ACTIVE_WINDOW must be positive")
	}
	return c, nil
}

func envList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		switch v {
		case "1", "true", "TRUE", "True", "yes", "YES", "Yes", "on", "ON", "On":
			return true
		case "0", "false", "FALSE", "False", "no", "NO", "No", "off", "OFF", "Off":
			return false
		}
	}
	return fallback
}
