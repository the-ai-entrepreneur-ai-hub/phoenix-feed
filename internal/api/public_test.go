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

func TestRootHandlerReturnsIdentifier(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	Router(&fakeStore{}, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["name"] != "cactus-watch-feed" || body["version"] != "v1" {
		t.Fatalf("root body = %#v", body)
	}
	if body["docs"] != "https://feed.cactuswatch.com/v1/openapi.json" {
		t.Fatalf("docs = %v", body["docs"])
	}
	if body["health"] != "https://feed.cactuswatch.com/v1/health" {
		t.Fatalf("health = %v", body["health"])
	}
}

func TestStatsEndpointReturnsPublicShape(t *testing.T) {
	st := &fakeStore{
		statsResult: store.PublicStats{
			CurrentActiveCount:  12,
			TodayTotalIncidents: 103,
			TodayByCategory: map[string]int{
				"Vehicle Crash":  47,
				"Structure Fire": 3,
			},
			Last24hTotal:    198,
			ActiveUnitsNow:  47,
			DataAgeSeconds:  32,
			SourceUpdatedAt: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC),
		},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["current_active_count"] != float64(12) {
		t.Fatalf("current_active_count = %v", body["current_active_count"])
	}
	if body["today_total_incidents"] != float64(103) {
		t.Fatalf("today_total_incidents = %v", body["today_total_incidents"])
	}
	if body["last_24h_total"] != float64(198) {
		t.Fatalf("last_24h_total = %v", body["last_24h_total"])
	}
	if body["active_units_now"] != float64(47) {
		t.Fatalf("active_units_now = %v", body["active_units_now"])
	}
	if body["data_age_seconds"] != float64(32) {
		t.Fatalf("data_age_seconds = %v", body["data_age_seconds"])
	}
	if body["tier"] != "free" {
		t.Fatalf("tier = %v", body["tier"])
	}
	today := body["today_by_category"].(map[string]any)
	if today["Vehicle Crash"] != float64(47) {
		t.Fatalf("Vehicle Crash count = %v", today["Vehicle Crash"])
	}
}

func TestCodesEndpointReturnsDictionary(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/codes", nil)

	Router(&fakeStore{}, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Version string `json:"version"`
		Codes   []struct {
			Code     string `json:"code"`
			Label    string `json:"label"`
			Category string `json:"category"`
		} `json:"codes"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Version != "phx-fire-2026-05" {
		t.Fatalf("version = %q", body.Version)
	}
	for _, code := range body.Codes {
		if code.Code == "962BC" {
			if code.Label != "Crash Involving Bicycle" || code.Category != "traffic" {
				t.Fatalf("962BC = %#v", code)
			}
			return
		}
	}
	t.Fatal("962BC not found in code dictionary")
}

func TestOpenAPIEndpointDocumentsRoutes(t *testing.T) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/openapi.json", nil)

	Router(&fakeStore{}, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["openapi"] != "3.0.3" {
		t.Fatalf("openapi = %v", body["openapi"])
	}
	paths := body["paths"].(map[string]any)
	for _, path := range []string{"/", "/api/admin/incidents/recent", "/v1/incidents/active", "/v1/fire-stations", "/v1/stats", "/v1/codes", "/v1/openapi.json", "/v1/health"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("path %s missing from openapi", path)
		}
	}
}
