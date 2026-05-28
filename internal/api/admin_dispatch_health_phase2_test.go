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

func TestAdminDispatchHealthIncludesParserFields(t *testing.T) {
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
	lastReceived := now.Add(-12 * time.Second)
	lastBatch := now.Add(-3 * time.Second)
	st := &fakeStore{dispatchHealth: store.DispatchTranscriptHealth{
		LastReceivedAt:                  &lastReceived,
		RowsLastHour:                    743,
		RowsLast24h:                     18234,
		HighConfidenceLastHour:          87,
		LowConfidenceLastHour:           656,
		ReviewRecommendedLastHour:       423,
		ParserLastBatchAt:               &lastBatch,
		ParserRowsPromotedLastHour:      4,
		ParserRowsGateFailedLastHour:    81,
		ParserRowsGeocodeFailedLastHour: 2,
		ParserBacklogUnparsed:           19,
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dispatch/health", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret", Now: func() time.Time { return now }}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["parser_last_batch_at"] != "2026-05-28T07:59:57Z" {
		t.Fatalf("parser_last_batch_at = %v", body["parser_last_batch_at"])
	}
	if body["parser_rows_promoted_last_hour"] != float64(4) {
		t.Fatalf("parser_rows_promoted_last_hour = %v", body["parser_rows_promoted_last_hour"])
	}
	if body["parser_rows_gate_failed_last_hour"] != float64(81) {
		t.Fatalf("parser_rows_gate_failed_last_hour = %v", body["parser_rows_gate_failed_last_hour"])
	}
	if body["parser_rows_geocode_failed_last_hour"] != float64(2) {
		t.Fatalf("parser_rows_geocode_failed_last_hour = %v", body["parser_rows_geocode_failed_last_hour"])
	}
	if body["parser_backlog_unparsed"] != float64(19) {
		t.Fatalf("parser_backlog_unparsed = %v", body["parser_backlog_unparsed"])
	}
}
