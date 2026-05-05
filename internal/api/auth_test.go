package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestAuthAllowsAnonymousFreeRequests(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("X-Client-ID", "ios-device-1")
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
}

func TestAuthAllowsValidAPIKey(t *testing.T) {
	key := "paid-secret"
	st := &fakeStore{
		apiKeys: map[string]store.APIKey{
			auth.HashKey(key): {ID: 7, Tier: "paid", Label: "owner"},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("X-API-Key", key)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.lookedUpHash != auth.HashKey(key) {
		t.Fatalf("looked up hash = %q", st.lookedUpHash)
	}
}

func TestAuthRejectsUnknownAPIKey(t *testing.T) {
	st := &fakeStore{apiKeys: map[string]store.APIKey{}}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("X-API-Key", "missing")
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuthRejectsRevokedAPIKey(t *testing.T) {
	key := "revoked-secret"
	revokedAt := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	st := &fakeStore{
		apiKeys: map[string]store.APIKey{
			auth.HashKey(key): {ID: 8, Tier: "paid", Label: "revoked", RevokedAt: &revokedAt},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("X-API-Key", key)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
