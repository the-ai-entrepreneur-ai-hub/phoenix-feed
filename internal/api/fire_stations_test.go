package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFireStationsEndpointReturnsProxiedFeatureServerJSON(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("where") != "1=1" {
			t.Fatalf("where = %q, want 1=1", r.URL.Query().Get("where"))
		}
		if r.URL.Query().Get("outFields") != "*" {
			t.Fatalf("outFields = %q, want *", r.URL.Query().Get("outFields"))
		}
		if r.URL.Query().Get("returnGeometry") != "true" {
			t.Fatalf("returnGeometry = %q, want true", r.URL.Query().Get("returnGeometry"))
		}
		if r.URL.Query().Get("outSR") != "4326" {
			t.Fatalf("outSR = %q, want 4326", r.URL.Query().Get("outSR"))
		}
		if r.URL.Query().Get("f") != "json" {
			t.Fatalf("f = %q, want json", r.URL.Query().Get("f"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"features":[{"attributes":{"STATION":"33"},"geometry":{"x":-112.074,"y":33.448}}]}`))
	}))
	defer upstream.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/fire-stations", nil)

	Router(&fakeStore{}, Config{
		FireStationsURL:        upstream.URL,
		FireStationsHTTPClient: upstream.Client(),
	}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("content-type = %q, want application/json", got)
	}
	var body struct {
		Features []json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Features) != 1 {
		t.Fatalf("features length = %d, want 1", len(body.Features))
	}
}

func TestFireStationsEndpointReturnsBadGatewayForUpstreamFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/fire-stations", nil)

	Router(&fakeStore{}, Config{
		FireStationsURL:        upstream.URL,
		FireStationsHTTPClient: upstream.Client(),
	}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502: %s", rr.Code, rr.Body.String())
	}
}
