package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterAddsCORSHeadersForFileClient(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "null")
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestRouterHandlesCORSPreflight(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodOptions, "/v1/incidents/active", nil)
	req.Header.Set("Origin", "null")
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got != "GET, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q", got)
	}
}
