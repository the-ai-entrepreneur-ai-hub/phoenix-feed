package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

type fakeStore struct {
	activeFilter store.ActiveIncidentFilter
	activeResult store.ActiveIncidentsResult
	activeErr    error
	recentFilter store.RecentIncidentFilter
	recentResult store.RecentIncidentsResult
	recentErr    error
	statsResult  store.PublicStats
	statsErr     error

	dispatchInsert          store.DispatchTranscriptInsert
	dispatchInsertID        int64
	dispatchInsertDuplicate bool
	dispatchInsertErr       error
	dispatchRecentLimit     int
	dispatchRecent          []store.DispatchTranscript
	dispatchRecentErr       error

	detailSource     string
	detailIncidentID string
	detailResult     store.IncidentDetailResult
	detailErr        error

	pingErr      error
	sourceHealth []store.SourceHealth
	healthErr    error

	apiKeys      map[string]store.APIKey
	lookedUpHash string
}

func (f *fakeStore) ListActiveIncidents(_ context.Context, filter store.ActiveIncidentFilter) (store.ActiveIncidentsResult, error) {
	f.activeFilter = filter
	return f.activeResult, f.activeErr
}

func (f *fakeStore) ListRecentIncidents(_ context.Context, filter store.RecentIncidentFilter) (store.RecentIncidentsResult, error) {
	f.recentFilter = filter
	return f.recentResult, f.recentErr
}

func (f *fakeStore) PublicStats(_ context.Context) (store.PublicStats, error) {
	return f.statsResult, f.statsErr
}

func (f *fakeStore) Ping(_ context.Context) error {
	return f.pingErr
}

func (f *fakeStore) SourceHealth(_ context.Context, _ []string) ([]store.SourceHealth, error) {
	return f.sourceHealth, f.healthErr
}

func (f *fakeStore) GetIncidentDetail(_ context.Context, source, incidentID string) (store.IncidentDetailResult, error) {
	f.detailSource = source
	f.detailIncidentID = incidentID
	return f.detailResult, f.detailErr
}

func (f *fakeStore) LookupAPIKey(_ context.Context, keyHash string) (store.APIKey, error) {
	f.lookedUpHash = keyHash
	key, ok := f.apiKeys[keyHash]
	if !ok || key.RevokedAt != nil {
		return store.APIKey{}, store.ErrAPIKeyNotFound
	}
	return key, nil
}

func TestActiveIncidentsEmptyResponseIncludesMeta(t *testing.T) {
	lastSuccess := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Meta: store.StalenessMeta{
				SourceLastSuccessAt: &lastSuccess,
				DataAgeSeconds:      ptrInt(45),
				ParserVersion:       "phx-fire-2026-05",
			},
			Incidents: []store.ActiveIncident{},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}

	var body struct {
		Meta      map[string]any `json:"meta"`
		Incidents []any          `json:"incidents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Meta["source_last_success_at"]; got != "2026-05-04T12:00:00Z" {
		t.Fatalf("source_last_success_at = %v", got)
	}
	if got := body.Meta["data_age_seconds"]; got != float64(45) {
		t.Fatalf("data_age_seconds = %v", got)
	}
	if got, ok := body.Meta["newest_incident_at"]; !ok || got != nil {
		t.Fatalf("newest_incident_at = %v, want null", got)
	}
	if got, ok := body.Meta["data_staleness_seconds"]; !ok || got != nil {
		t.Fatalf("data_staleness_seconds = %v, want null", got)
	}
	if got := body.Meta["parser_version"]; got != "phx-fire-2026-05" {
		t.Fatalf("parser_version = %v", got)
	}
	if len(body.Incidents) != 0 {
		t.Fatalf("incidents length = %d, want 0", len(body.Incidents))
	}
}

func TestActiveIncidentsMetaUsesNewestReturnedIncidentDate(t *testing.T) {
	now := time.Date(2026, 5, 26, 1, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 5, 26, 0, 59, 15, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Meta: store.StalenessMeta{
				SourceLastSuccessAt: &lastSuccess,
				DataAgeSeconds:      ptrInt(45),
				ParserVersion:       "phx-fire-2026-05",
			},
			Incidents: []store.ActiveIncident{
				{IncidentID: "older-1", Units: []model.Unit{}, IncidentDate: time.Date(2026, 5, 25, 23, 30, 0, 0, time.UTC)},
				{IncidentID: "newest", Units: []model.Unit{}, IncidentDate: time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)},
				{IncidentID: "older-2", Units: []model.Unit{}, IncidentDate: time.Date(2026, 5, 25, 22, 0, 0, 0, time.UTC)},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Meta["newest_incident_at"]; got != "2026-05-26T00:00:00Z" {
		t.Fatalf("newest_incident_at = %v", got)
	}
	if got := body.Meta["data_staleness_seconds"]; got != float64(3600) {
		t.Fatalf("data_staleness_seconds = %v", got)
	}
	if got := body.Meta["source_last_success_at"]; got != "2026-05-26T00:59:15Z" {
		t.Fatalf("source_last_success_at = %v", got)
	}
	if got := body.Meta["data_age_seconds"]; got != float64(45) {
		t.Fatalf("data_age_seconds = %v", got)
	}
}

func TestActiveIncidentsMetaStalenessIsZeroWhenNewestEqualsNow(t *testing.T) {
	now := time.Date(2026, 5, 26, 1, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Incidents: []store.ActiveIncident{{
				IncidentID:   "current",
				Units:        []model.Unit{},
				IncidentDate: now,
			}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Meta["newest_incident_at"]; got != "2026-05-26T01:00:00Z" {
		t.Fatalf("newest_incident_at = %v", got)
	}
	if got := body.Meta["data_staleness_seconds"]; got != float64(0) {
		t.Fatalf("data_staleness_seconds = %v", got)
	}
}

func TestActiveIncidentsEmptyUnitsSerializesAsArray(t *testing.T) {
	incidentDate := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Incidents: []store.ActiveIncident{{
				Source:       "phoenix-fire-mapserver",
				IncidentID:   "F26200326",
				NatureCode:   "STR",
				NatureDesc:   "STRUCTURE FIRE",
				Units:        []model.Unit{},
				Lon:          -112.074,
				Lat:          33.448,
				IncidentDate: incidentDate,
				ReceivedAt:   incidentDate,
				LastSeenAt:   incidentDate,
			}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	incidents := body["incidents"].([]any)
	first := incidents[0].(map[string]any)
	units, ok := first["units"].([]any)
	if !ok {
		t.Fatalf("units = %#v, want empty JSON array", first["units"])
	}
	if len(units) != 0 {
		t.Fatalf("units length = %d, want 0", len(units))
	}
}

func TestActiveIncidentsAddsSeverity(t *testing.T) {
	incidentDate := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Incidents: []store.ActiveIncident{{
				Source:       "phoenix-fire-mapserver",
				IncidentID:   "F26200331",
				NatureCode:   "962X",
				NatureDesc:   "Crash Requiring Extrication",
				Units:        []model.Unit{},
				Lon:          -112.074,
				Lat:          33.448,
				IncidentDate: incidentDate,
				ReceivedAt:   incidentDate,
				LastSeenAt:   incidentDate,
			}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	first := body["incidents"].([]any)[0].(map[string]any)
	if got := first["severity"]; got != "medium" {
		t.Fatalf("severity = %v, want medium", got)
	}
}

func TestActiveIncidentsKeepsLongRunningObservedIncident(t *testing.T) {
	now := time.Date(2026, 5, 18, 6, 2, 0, 0, time.UTC)
	dispatchedAt := now.Add(-2*time.Hour - 23*time.Minute)
	lastSeenAt := now.Add(-45 * time.Second)
	st := &fakeStore{
		activeResult: store.ActiveIncidentsResult{
			Incidents: []store.ActiveIncident{{
				Source:               "phoenix-fire-mapserver",
				IncidentID:           "F26209999",
				NatureCode:           "MTNRES",
				NatureDesc:           "Mountain Rescue",
				Units:                []model.Unit{{Unit: "E611", Status: "Dispatched"}},
				Lon:                  -112.074,
				Lat:                  33.448,
				IncidentDate:         dispatchedAt,
				ReceivedAt:           dispatchedAt,
				LastSeenAt:           lastSeenAt,
				SecondsSinceLastSeen: 45,
			}},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	incidents := body["incidents"].([]any)
	if len(incidents) != 1 {
		t.Fatalf("incidents length = %d, want 1", len(incidents))
	}
	first := incidents[0].(map[string]any)
	if got := first["incident_id"]; got != "F26209999" {
		t.Fatalf("incident_id = %v, want F26209999", got)
	}
	if got := first["seconds_since_last_seen"]; got != float64(45) {
		t.Fatalf("seconds_since_last_seen = %v, want 45", got)
	}
}

func TestActiveIncidentsParsesBBoxFilter(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active?bbox=-112.2,33.2,-111.7,33.8", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.activeFilter.BBox == nil {
		t.Fatal("bbox filter was not set")
	}
	want := store.BBox{MinLon: -112.2, MinLat: 33.2, MaxLon: -111.7, MaxLat: 33.8}
	if *st.activeFilter.BBox != want {
		t.Fatalf("bbox = %+v, want %+v", *st.activeFilter.BBox, want)
	}
}

func TestActiveIncidentsParsesRadiusAndTimeWindow(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active?lat=33.45&lon=-112.07&radius_meters=2500&since=2026-05-04T10:00:00Z&until=2026-05-04T11:00:00Z", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.activeFilter.Radius == nil {
		t.Fatal("radius filter was not set")
	}
	if st.activeFilter.Radius.Lat != 33.45 || st.activeFilter.Radius.Lon != -112.07 || st.activeFilter.Radius.Meters != 2500 {
		t.Fatalf("radius = %+v", *st.activeFilter.Radius)
	}
	if st.activeFilter.Since == nil || st.activeFilter.Since.Format(time.RFC3339) != "2026-05-04T10:00:00Z" {
		t.Fatalf("since = %v", st.activeFilter.Since)
	}
	if st.activeFilter.Until == nil || st.activeFilter.Until.Format(time.RFC3339) != "2026-05-04T11:00:00Z" {
		t.Fatalf("until = %v", st.activeFilter.Until)
	}
}

func TestActiveIncidentsRejectsMixedSpatialFilters(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active?bbox=-112,33,-111,34&lat=33.45&lon=-112.07&radius_meters=2500", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func ptrInt(v int) *int {
	return &v
}

func ptrBool(v bool) *bool {
	return &v
}

var errTestPing = errors.New("db unavailable")
