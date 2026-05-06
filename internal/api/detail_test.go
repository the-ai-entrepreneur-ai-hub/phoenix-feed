package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestIncidentDetailFound(t *testing.T) {
	incidentDate := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	lastSuccess := time.Date(2026, 5, 4, 10, 1, 0, 0, time.UTC)
	st := &fakeStore{
		detailResult: store.IncidentDetailResult{
			Meta: store.StalenessMeta{
				SourceLastSuccessAt: &lastSuccess,
				DataAgeSeconds:      ptrInt(30),
				ParserVersion:       "phx-fire-2026-05",
			},
			Incident: &store.IncidentDetail{
				Source:       "phoenix-fire-mapserver",
				IncidentID:   "F26198635",
				NatureCode:   "WF",
				NatureDesc:   "REPORTD WORKING FIRE",
				LocationText: "600 S COUNTRY CLUB DR ,MES",
				Lon:          -111.84006,
				Lat:          33.40284,
				IncidentDate: incidentDate,
			},
			UnitHistory: []store.UnitObservation{{
				Unit:            "E2203",
				Status:          "On Scene",
				FirstObservedAt: incidentDate,
				LastObservedAt:  incidentDate.Add(time.Minute),
			}},
			Events: []store.IncidentEvent{{
				EventType:  "created",
				OccurredAt: incidentDate,
			}},
		},
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/phoenix-fire-mapserver/F26198635", nil)

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.detailSource != "phoenix-fire-mapserver" || st.detailIncidentID != "F26198635" {
		t.Fatalf("lookup = %s/%s", st.detailSource, st.detailIncidentID)
	}

	var body struct {
		Incident    map[string]any `json:"incident"`
		UnitHistory []any          `json:"unit_history"`
		Events      []any          `json:"events"`
		Meta        map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Incident["incident_id"] != "F26198635" {
		t.Fatalf("incident_id = %v", body.Incident["incident_id"])
	}
	if body.Incident["severity"] != "high" {
		t.Fatalf("severity = %v, want high", body.Incident["severity"])
	}
	if len(body.UnitHistory) != 1 {
		t.Fatalf("unit_history length = %d", len(body.UnitHistory))
	}
	if len(body.Events) != 1 {
		t.Fatalf("events length = %d", len(body.Events))
	}
	if body.Meta["parser_version"] != "phx-fire-2026-05" {
		t.Fatalf("parser_version = %v", body.Meta["parser_version"])
	}
}

func TestIncidentDetailMissing(t *testing.T) {
	st := &fakeStore{detailResult: store.IncidentDetailResult{}}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/phoenix-fire-mapserver/NOPE", nil)

	Router(st, Config{}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}
