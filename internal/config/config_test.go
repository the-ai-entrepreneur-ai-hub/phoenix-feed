package config

import (
	"testing"
	"time"
)

func TestLoadDispatchMaxAgeDefaultsTwoHours(t *testing.T) {
	t.Setenv("DISPATCH_MAX_AGE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DispatchMaxAge != 2*time.Hour {
		t.Fatalf("DispatchMaxAge = %s, want 2h", cfg.DispatchMaxAge)
	}
}

func TestLoadDispatchMaxAgeFromEnv(t *testing.T) {
	t.Setenv("DISPATCH_MAX_AGE", "45m")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.DispatchMaxAge != 45*time.Minute {
		t.Fatalf("DispatchMaxAge = %s, want 45m", cfg.DispatchMaxAge)
	}
}

func TestLoadSDRActiveWindowDefaultsNinetyMinutes(t *testing.T) {
	t.Setenv("SDR_ACTIVE_WINDOW", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SDRActiveWindow != 90*time.Minute {
		t.Fatalf("SDRActiveWindow = %s, want 90m", cfg.SDRActiveWindow)
	}
}

func TestLoadSDRActiveWindowFromEnv(t *testing.T) {
	t.Setenv("SDR_ACTIVE_WINDOW", "2h")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.SDRActiveWindow != 2*time.Hour {
		t.Fatalf("SDRActiveWindow = %s, want 2h", cfg.SDRActiveWindow)
	}
}

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

func TestLoadAdminTokenFromEnv(t *testing.T) {
	t.Setenv("ADMIN_TOKEN", "admin-secret")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AdminToken != "admin-secret" {
		t.Fatalf("AdminToken = %q, want admin-secret", cfg.AdminToken)
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

func TestLoadSourceHealthThresholdDefaults(t *testing.T) {
	t.Setenv("SOURCE_DEGRADED_AFTER", "")
	t.Setenv("SOURCE_DOWN_AFTER", "")
	t.Setenv("SOURCE_DOWN_FAILURES", "")
	t.Setenv("FROZEN_REPEAT_COUNT", "")
	t.Setenv("FROZEN_DOWN_AFTER", "")
	t.Setenv("SUDDEN_COLLAPSE_PERCENT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceDegradedAfter != 3*time.Minute || cfg.SourceDownAfter != 10*time.Minute || cfg.SourceDownFailures != 3 {
		t.Fatalf("source thresholds = %s/%s/%d", cfg.SourceDegradedAfter, cfg.SourceDownAfter, cfg.SourceDownFailures)
	}
	if cfg.FrozenRepeatCount != 3 || cfg.FrozenDownAfter != 10*time.Minute || cfg.SuddenCollapsePct != 80 {
		t.Fatalf("snapshot thresholds = %d/%s/%d", cfg.FrozenRepeatCount, cfg.FrozenDownAfter, cfg.SuddenCollapsePct)
	}
}

func TestLoadRejectsInvalidSourceHealthThresholds(t *testing.T) {
	t.Setenv("SOURCE_DEGRADED_AFTER", "10m")
	t.Setenv("SOURCE_DOWN_AFTER", "3m")
	if _, err := Load(); err == nil {
		t.Fatal("Load succeeded with degraded threshold after down threshold")
	}
}
