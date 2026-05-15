package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestActiveIncidentsAllowsRepeatedCachedReads(t *testing.T) {
	st := &fakeStore{}
	limiter := ratelimit.New(ratelimit.Config{
		FreeEvery:   10 * time.Minute,
		PaidEvery:   50 * time.Second,
		ManualEvery: 120 * time.Second,
	})
	router := Router(st, Config{RateLimiter: limiter}, slog.Default())

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	req.Header.Set("X-Client-ID", "device-rate-test")
	router.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200", first.Code)
	}

	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
		req.Header.Set("X-Client-ID", "device-rate-test")
		router.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("repeat %d status = %d, want 200: %s", i+1, rr.Code, rr.Body.String())
		}
		if got := rr.Header().Get("Retry-After"); got != "" {
			t.Fatalf("repeat %d Retry-After = %q, want empty", i+1, got)
		}
	}
}

func TestHealthBypassesIncidentRateLimit(t *testing.T) {
	lastSuccess := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		sourceHealth: []store.SourceHealth{{
			Source:        phxfire.SourceName,
			LastSuccessAt: &lastSuccess,
			ParserVersion: phxfire.ParserVersion,
		}},
	}
	limiter := ratelimit.New(ratelimit.Config{
		FreeEvery:   10 * time.Minute,
		PaidEvery:   50 * time.Second,
		ManualEvery: 120 * time.Second,
	})
	router := Router(st, Config{
		RateLimiter: limiter,
		Sources:     []string{phxfire.SourceName},
		Now:         func() time.Time { return lastSuccess.Add(time.Minute) },
	}, slog.Default())

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
		req.Header.Set("X-Client-ID", "device-health-test")
		router.ServeHTTP(rr, req)
	}

	health := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.Header.Set("X-Client-ID", "device-health-test")
	router.ServeHTTP(health, req)

	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200: %s", health.Code, health.Body.String())
	}
}
