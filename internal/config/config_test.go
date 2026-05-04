package config

import "testing"

func TestLoadPaidTierDefaultsFalse(t *testing.T) {
	t.Setenv("PAID_TIER_ENABLED", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.PaidTierEnabled {
		t.Fatal("PaidTierEnabled = true, want false")
	}
}

func TestLoadPaidTierEnabledFromEnv(t *testing.T) {
	t.Setenv("PAID_TIER_ENABLED", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if !cfg.PaidTierEnabled {
		t.Fatal("PaidTierEnabled = false, want true")
	}
}
