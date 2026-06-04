package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestActiveMetaForFreeClient(t *testing.T) {
	body := requestIncidentPayload(t, "", http.MethodGet, "/v1/incidents/active")

	meta := body["meta"].(map[string]any)
	if meta["disclaimer"] != "Not for emergency use; call 911" {
		t.Fatalf("disclaimer = %v", meta["disclaimer"])
	}
	// Attribution is intentionally empty (omitted via omitempty): no
	// source-agency naming is allowed anywhere user-facing.
	if _, present := meta["attribution"]; present {
		t.Fatalf("attribution should be omitted, got %v", meta["attribution"])
	}
	if meta["refresh_min_seconds"] != float64(60) {
		t.Fatalf("refresh_min_seconds = %v", meta["refresh_min_seconds"])
	}
	if meta["tier"] != "free" {
		t.Fatalf("tier = %v", meta["tier"])
	}
}

func TestActiveMetaForPaidClient(t *testing.T) {
	body := requestIncidentPayload(t, "paid-meta-key", http.MethodGet, "/v1/incidents/active")

	meta := body["meta"].(map[string]any)
	if meta["refresh_min_seconds"] != float64(60) {
		t.Fatalf("refresh_min_seconds = %v", meta["refresh_min_seconds"])
	}
	if meta["tier"] != "paid" {
		t.Fatalf("tier = %v", meta["tier"])
	}
}

func TestRefreshMetaUsesManualCadence(t *testing.T) {
	body := requestIncidentPayload(t, "", http.MethodPost, "/v1/incidents/refresh")

	meta := body["meta"].(map[string]any)
	if meta["refresh_min_seconds"] != float64(120) {
		t.Fatalf("refresh_min_seconds = %v", meta["refresh_min_seconds"])
	}
	if meta["tier"] != "free" {
		t.Fatalf("tier = %v", meta["tier"])
	}
}

func requestIncidentPayload(t *testing.T, apiKey string, method string, path string) map[string]any {
	t.Helper()
	lastSuccess := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Meta: store.StalenessMeta{
				SourceLastSuccessAt: &lastSuccess,
				DataAgeSeconds:      ptrInt(5),
				ParserVersion:       "phx-fire-2026-05",
			},
			Incidents: []store.ActiveIncident{},
		},
		apiKeys: map[string]store.APIKey{},
	}
	if apiKey != "" {
		st.apiKeys[auth.HashKey(apiKey)] = store.APIKey{ID: 99, Tier: "paid", Label: "paid"}
	}

	router := Router(st, Config{
		RateLimiter: ratelimit.New(ratelimit.Config{
			FreeEvery:   10 * time.Minute,
			PaidEvery:   50 * time.Second,
			ManualEvery: 120 * time.Second,
		}),
	}, slog.Default())

	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("X-Client-ID", "meta-device-"+method+path)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}
