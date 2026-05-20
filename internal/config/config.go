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
	DatabaseURL      string        // postgres://...
	AdminToken       string        // bearer token for private admin endpoints
	PollInterval     time.Duration // ingester base cadence
	PollJitter       time.Duration // ± jitter applied each cycle
	HTTPAddr         string        // api bind address
	FireStationsURL  string        // optional upstream override for station overlay proxy
	ClearAfterMisses int           // consecutive successful polls absent before clearing
	RawRetention     time.Duration // janitor: drop raw JSONB after this age
	LogLevel         string        // "debug" | "info" | "warn" | "error"
	PaidTierEnabled  bool          // gates paid placeholders; default false
	AllowedOrigins   []string      // exact browser origins allowed for CORS
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:      env("DATABASE_URL", "postgres://localhost:5432/phoenix_feed?sslmode=disable"),
		AdminToken:       env("ADMIN_TOKEN", ""),
		HTTPAddr:         env("HTTP_ADDR", ":8080"),
		FireStationsURL:  env("FIRE_STATIONS_URL", ""),
		LogLevel:         env("LOG_LEVEL", "info"),
		ClearAfterMisses: envInt("CLEAR_AFTER_MISSES", 5),
		PollInterval:     envDuration("POLL_INTERVAL", 60*time.Second),
		PollJitter:       envDuration("POLL_JITTER", 10*time.Second),
		RawRetention:     envDuration("RAW_RETENTION", 30*24*time.Hour),
		PaidTierEnabled:  envBool("PAID_TIER_ENABLED", false),
		AllowedOrigins:   envList("ALLOWED_ORIGINS"),
	}

	if c.PollJitter >= c.PollInterval {
		return c, fmt.Errorf("POLL_JITTER (%s) must be less than POLL_INTERVAL (%s)", c.PollJitter, c.PollInterval)
	}
	if c.ClearAfterMisses < 1 {
		return c, fmt.Errorf("CLEAR_AFTER_MISSES must be >= 1")
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
