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

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

type fakeStore struct {
	activeFilter store.ActiveIncidentFilter
	activeResult store.ActiveIncidentsResult
	activeErr    error

	detailSource     string
	detailIncidentID string
	detailResult     store.IncidentDetailResult
	detailErr        error

	pingErr      error
	sourceHealth []store.SourceHealth
	healthErr    error
}

func (f *fakeStore) ListActiveIncidents(_ context.Context, filter store.ActiveIncidentFilter) (store.ActiveIncidentsResult, error) {
	f.activeFilter = filter
	return f.activeResult, f.activeErr
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
	if got := body.Meta["parser_version"]; got != "phx-fire-2026-05" {
		t.Fatalf("parser_version = %v", got)
	}
	if len(body.Incidents) != 0 {
		t.Fatalf("incidents length = %d, want 0", len(body.Incidents))
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
