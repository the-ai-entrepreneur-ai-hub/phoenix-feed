package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHistoryPlaceholderReturnsPaymentRequiredWhenPaidTierDisabled(t *testing.T) {
	st := &fakeStore{}
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/history", nil)
	rr := httptest.NewRecorder()

	Router(st, Config{PaidTierEnabled: false}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "paid history is not enabled for v0.2" {
		t.Fatalf("error = %v", body["error"])
	}
}
