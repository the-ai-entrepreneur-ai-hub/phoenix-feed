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

func TestLoadAllowedOriginsDefaultsEmpty(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.AllowedOrigins) != 0 {
		t.Fatalf("AllowedOrigins = %#v, want empty", cfg.AllowedOrigins)
	}
}

func TestLoadAllowedOriginsFromCommaList(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://cactuswatch.example, cactuswatch://app ,")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"https://cactuswatch.example", "cactuswatch://app"}
	if len(cfg.AllowedOrigins) != len(want) {
		t.Fatalf("AllowedOrigins = %#v, want %#v", cfg.AllowedOrigins, want)
	}
	for i := range want {
		if cfg.AllowedOrigins[i] != want[i] {
			t.Fatalf("AllowedOrigins[%d] = %q, want %q", i, cfg.AllowedOrigins[i], want[i])
		}
	}
}
