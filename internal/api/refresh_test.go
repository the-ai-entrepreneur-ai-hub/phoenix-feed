package api

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestRefreshReturnsActiveCacheShape(t *testing.T) {
	lastSuccess := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Meta: store.StalenessMeta{
				SourceLastSuccessAt: &lastSuccess,
				DataAgeSeconds:      ptrInt(10),
				ParserVersion:       "phx-fire-2026-05",
			},
			Incidents: []store.ActiveIncident{},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/incidents/refresh?bbox=-112,33,-111,34", nil)
	req.Header.Set("X-Client-ID", "refresh-device")
	rr := httptest.NewRecorder()

	Router(st, Config{RateLimiter: ratelimit.NewDefault()}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.activeFilter.BBox == nil {
		t.Fatal("refresh did not reuse active filter parsing")
	}
}

func TestRefreshSecondRequestIsThrottled(t *testing.T) {
	st := &fakeStore{}
	limiter := ratelimit.New(ratelimit.Config{
		FreeEvery:   10 * time.Minute,
		PaidEvery:   50 * time.Second,
		ManualEvery: 120 * time.Second,
	})
	router := Router(st, Config{RateLimiter: limiter}, slog.Default())

	for i := 0; i < 2; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/v1/incidents/refresh", nil)
		req.Header.Set("X-Client-ID", "manual-refresh-device")
		router.ServeHTTP(rr, req)
		if i == 0 && rr.Code != http.StatusOK {
			t.Fatalf("first status = %d, want 200", rr.Code)
		}
		if i == 1 {
			if rr.Code != http.StatusTooManyRequests {
				t.Fatalf("second status = %d, want 429", rr.Code)
			}
			if got := rr.Header().Get("Retry-After"); got == "" {
				t.Fatal("Retry-After header missing")
			}
		}
	}
}
