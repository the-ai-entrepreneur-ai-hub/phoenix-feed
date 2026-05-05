// Command keygen creates a manual API key and stores only its hash.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	tierRaw := env("KEY_TIER", "paid")
	tier, ok := auth.TierFromString(tierRaw)
	if !ok {
		fatal("tier", fmt.Errorf("KEY_TIER must be free or paid"))
	}

	key, err := auth.GenerateKey()
	if err != nil {
		fatal("generate key", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	st, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("store", err)
	}
	defer st.Close()

	label := env("KEY_LABEL", "manual")
	ownerEmail := env("OWNER_EMAIL", "")
	record, err := st.CreateAPIKey(ctx, store.CreateAPIKeyParams{
		KeyHash:    auth.HashKey(key),
		Tier:       string(tier),
		Label:      label,
		OwnerEmail: ownerEmail,
	})
	if err != nil {
		fatal("create api key", err)
	}

	fmt.Printf("api_key=%s\n", key)
	fmt.Printf("id=%d\n", record.ID)
	fmt.Printf("tier=%s\n", record.Tier)
	fmt.Printf("label=%s\n", record.Label)
	if record.OwnerEmail != "" {
		fmt.Printf("owner_email=%s\n", record.OwnerEmail)
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fatal(msg string, err error) {
	slog.Error(msg, "err", err)
	os.Exit(1)
}
