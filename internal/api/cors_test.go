package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterAllowsNativeRequestWithoutCORSHeader(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestRouterAllowsConfiguredCORSOrigin(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "https://cactuswatch.example")
	rr := httptest.NewRecorder()

	Router(st, Config{AllowedOrigins: []string{"https://cactuswatch.example"}}, slog.Default()).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://cactuswatch.example" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := rr.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("Vary = %q", got)
	}
}

func TestRouterDeniesUnconfiguredCORSOrigin(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rr := httptest.NewRecorder()

	Router(st, Config{AllowedOrigins: []string{"https://cactuswatch.example"}}, slog.Default()).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestRouterHandlesAllowedCORSPreflight(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodOptions, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "https://cactuswatch.example")
	rr := httptest.NewRecorder()

	Router(st, Config{AllowedOrigins: []string{"https://cactuswatch.example"}}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
}

func TestRouterDeniesUnconfiguredCORSPreflight(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodOptions, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "https://attacker.example")
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}
