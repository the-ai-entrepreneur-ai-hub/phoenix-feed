package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestAdminRecentIncidentsRequiresBearerToken(t *testing.T) {
	for _, tc := range []struct {
		name          string
		authorization string
	}{
		{name: "missing", authorization: ""},
		{name: "invalid", authorization: "Bearer wrong"},
		{name: "wrong scheme", authorization: "Basic admin-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/incidents/recent?hours=48", nil)
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rr := httptest.NewRecorder()

			Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminRecentIncidentsRejectsInvalidHours(t *testing.T) {
	for _, hours := range []string{"0", "-1", "49", "abc"} {
		t.Run(hours, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/admin/incidents/recent?hours="+hours, nil)
			req.Header.Set("Authorization", "Bearer admin-secret")
			rr := httptest.NewRecorder()

			Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminRecentIncidentsReturnsRecentArray(t *testing.T) {
	now := time.Date(2026, 5, 19, 20, 0, 0, 0, time.UTC)
	incidentTime := now.Add(-2 * time.Hour)
	st := &fakeStore{
		recentResult: store.RecentIncidentsResult{
			TotalCount: 2,
			Incidents: []store.ActiveIncident{{
				Source:       "phoenix-fire-mapserver",
				IncidentID:   "F26201000",
				NatureCode:   "962X",
				NatureDesc:   "Crash Requiring Extrication",
				Units:        []model.Unit{},
				Lon:          -112.074,
				Lat:          33.448,
				IncidentDate: incidentTime,
				ReceivedAt:   incidentTime,
				LastSeenAt:   incidentTime,
			}},
		},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/incidents/recent?hours=48", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.recentFilter.Hours != 48 || st.recentFilter.Limit != 500 {
		t.Fatalf("recent filter = %+v, want hours=48 limit=500", st.recentFilter)
	}
	if got := rr.Header().Get("X-Total-Count"); got != "2" {
		t.Fatalf("X-Total-Count = %q, want 2", got)
	}
	var body []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 {
		t.Fatalf("body length = %d, want 1", len(body))
	}
	if body[0]["incident_id"] != "F26201000" {
		t.Fatalf("incident_id = %v", body[0]["incident_id"])
	}
	if body[0]["severity"] != "medium" {
		t.Fatalf("severity = %v, want medium", body[0]["severity"])
	}
}
