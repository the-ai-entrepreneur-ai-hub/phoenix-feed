package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestHealthOKWhenSourceFreshAndCanaryPassing(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	lastSuccess := now.Add(-2 * time.Minute)
	checkedAt := now.Add(-1 * time.Minute)
	st := &fakeStore{
		sourceHealth: []store.SourceHealth{{
			Source:        "phoenix-fire-mapserver",
			LastSuccessAt: &lastSuccess,
			ParserVersion: "phx-fire-2026-05",
			Canary: &store.CanaryHealth{
				CheckedAt: &checkedAt,
				Passed:    ptrBool(true),
			},
		}},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	Router(st, Config{Sources: []string{"phoenix-fire-mapserver"}, Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["state"] != "ok" {
		t.Fatalf("state = %v", body["state"])
	}
	sources := body["sources"].(map[string]any)
	phoenix := sources["phoenix-fire-mapserver"].(map[string]any)
	if phoenix["seconds_since_success"] != float64(120) {
		t.Fatalf("seconds_since_success = %v", phoenix["seconds_since_success"])
	}
}

func TestHealthAllowsHeadProbe(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	lastSuccess := now.Add(-2 * time.Minute)
	st := &fakeStore{
		sourceHealth: []store.SourceHealth{{
			Source:        "phoenix-fire-mapserver",
			LastSuccessAt: &lastSuccess,
			ParserVersion: "phx-fire-2026-05",
		}},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodHead, "/v1/health", nil)

	Router(st, Config{Sources: []string{"phoenix-fire-mapserver"}, Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
}

func TestHealthDegradedWhenCanaryFailsButSourceFresh(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	lastSuccess := now.Add(-2 * time.Minute)
	checkedAt := now.Add(-1 * time.Minute)
	st := &fakeStore{
		sourceHealth: []store.SourceHealth{{
			Source:        "phoenix-fire-mapserver",
			LastSuccessAt: &lastSuccess,
			ParserVersion: "phx-fire-2026-05",
			Canary: &store.CanaryHealth{
				CheckedAt: &checkedAt,
				Passed:    ptrBool(false),
			},
		}},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	Router(st, Config{Sources: []string{"phoenix-fire-mapserver"}, Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["state"] != "degraded" {
		t.Fatalf("state = %v", body["state"])
	}
}

func TestHealthDownWhenSourceIsStale(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 10, 0, 0, time.UTC)
	lastSuccess := now.Add(-11 * time.Minute)
	st := &fakeStore{
		sourceHealth: []store.SourceHealth{{
			Source:        "phoenix-fire-mapserver",
			LastSuccessAt: &lastSuccess,
			ParserVersion: "phx-fire-2026-05",
		}},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	Router(st, Config{Sources: []string{"phoenix-fire-mapserver"}, Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["state"] != "down" {
		t.Fatalf("state = %v", body["state"])
	}
}

func TestHealthDownWhenDatabasePingFails(t *testing.T) {
	st := &fakeStore{pingErr: errTestPing}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)

	Router(st, Config{Sources: []string{"phoenix-fire-mapserver"}}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rr.Code)
	}
}
