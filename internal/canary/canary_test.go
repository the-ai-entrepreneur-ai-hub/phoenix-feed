package canary

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

type fakeCanaryStore struct {
	baseline            float64
	hasBaseline         bool
	recentFeatureCounts []int
	recorded            []store.ContractCanaryResult
	recordedError       error
}

func (f *fakeCanaryStore) FeatureCountBaseline(_ context.Context, _ string, _ time.Time) (float64, bool, error) {
	return f.baseline, f.hasBaseline, nil
}

func (f *fakeCanaryStore) RecentCanaryFeatureCounts(_ context.Context, _ string, _ int) ([]int, error) {
	return f.recentFeatureCounts, nil
}

func (f *fakeCanaryStore) RecordCanary(_ context.Context, result store.ContractCanaryResult) error {
	f.recorded = append(f.recorded, result)
	return f.recordedError
}

func TestCheckPassesValidPhoenixFireResponse(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(jsonHandler(validCanaryBody(now.Add(-time.Hour).UnixMilli())))
	defer srv.Close()

	checker := Checker{
		URL:    srv.URL,
		Store:  &fakeCanaryStore{},
		Client: srv.Client(),
		Now:    func() time.Time { return now },
		Log:    slog.Default(),
	}

	result, err := checker.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("passed = false, drift = %s", string(result.Drift))
	}
	if result.OutputSR == nil || *result.OutputSR != 4326 {
		t.Fatalf("output_sr = %v", result.OutputSR)
	}
	if result.FeatureCount != 1 {
		t.Fatalf("feature_count = %d", result.FeatureCount)
	}
}

func TestCheckFailsMissingRequiredField(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	body := strings.Replace(validCanaryBody(now.Add(-time.Hour).UnixMilli()), `{"name":"Incident"},`, "", 1)
	srv := httptest.NewServer(jsonHandler(body))
	defer srv.Close()

	result, err := Checker{URL: srv.URL, Store: &fakeCanaryStore{}, Client: srv.Client(), Now: func() time.Time { return now }, Log: slog.Default()}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("passed = true, want false")
	}
	if !strings.Contains(string(result.Drift), "missing_fields") {
		t.Fatalf("drift = %s", string(result.Drift))
	}
}

func TestCheckFailsBadGeometry(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	body := strings.Replace(validCanaryBody(now.Add(-time.Hour).UnixMilli()), `"x":-111.84006,"y":33.40284`, `"x":700000,"y":900000`, 1)
	srv := httptest.NewServer(jsonHandler(body))
	defer srv.Close()

	result, err := Checker{URL: srv.URL, Store: &fakeCanaryStore{}, Client: srv.Client(), Now: func() time.Time { return now }, Log: slog.Default()}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("passed = true, want false")
	}
	if !strings.Contains(string(result.Drift), "geometry") {
		t.Fatalf("drift = %s", string(result.Drift))
	}
}

func TestCheckFailsBadDate(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(jsonHandler(validCanaryBody(now.Add(24 * time.Hour).UnixMilli())))
	defer srv.Close()

	result, err := Checker{URL: srv.URL, Store: &fakeCanaryStore{}, Client: srv.Client(), Now: func() time.Time { return now }, Log: slog.Default()}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("passed = true, want false")
	}
	if !strings.Contains(string(result.Drift), "date_window") {
		t.Fatalf("drift = %s", string(result.Drift))
	}
}

func TestCheckNotesSingleZeroFeatureCount(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(jsonHandler(zeroFeatureCanaryBody()))
	defer srv.Close()

	result, err := Checker{
		URL:    srv.URL,
		Store:  &fakeCanaryStore{baseline: 10, hasBaseline: true},
		Client: srv.Client(),
		Now:    func() time.Time { return now },
		Log:    slog.Default(),
	}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("passed = false, drift = %s", string(result.Drift))
	}
	if !strings.Contains(string(result.Drift), "feature_count_zero") {
		t.Fatalf("drift = %s, want informational feature_count_zero note", string(result.Drift))
	}
}

func TestCheckFailsAfterThreeConsecutiveZeroFeatureCounts(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(jsonHandler(zeroFeatureCanaryBody()))
	defer srv.Close()

	result, err := Checker{
		URL:    srv.URL,
		Store:  &fakeCanaryStore{baseline: 10, hasBaseline: true, recentFeatureCounts: []int{0, 0}},
		Client: srv.Client(),
		Now:    func() time.Time { return now },
		Log:    slog.Default(),
	}.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("passed = true, want false after three consecutive zero-feature checks")
	}
	if !strings.Contains(string(result.Drift), "feature_count") {
		t.Fatalf("drift = %s", string(result.Drift))
	}
}

func TestRunRecordsCanaryResult(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(jsonHandler(validCanaryBody(now.Add(-time.Hour).UnixMilli())))
	defer srv.Close()
	st := &fakeCanaryStore{}

	checker := Checker{URL: srv.URL, Store: st, Client: srv.Client(), Now: func() time.Time { return now }, Log: slog.Default()}
	if _, err := checker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(st.recorded) != 1 {
		t.Fatalf("recorded = %d, want 1", len(st.recorded))
	}
	if st.recorded[0].Source != "phoenix-fire-mapserver" {
		t.Fatalf("source = %q", st.recorded[0].Source)
	}
}

func jsonHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func validCanaryBody(dateMillis int64) string {
	body := map[string]any{
		"fields": []map[string]string{
			{"name": "OBJECTID"},
			{"name": "Incident"},
			{"name": "Nature"},
			{"name": "NatureDesc"},
			{"name": "Units"},
			{"name": "Channel"},
			{"name": "SymbolCode"},
			{"name": "Date"},
			{"name": "GenLocInfo"},
		},
		"spatialReference": map[string]int{"wkid": 4326},
		"features": []map[string]any{{
			"attributes": map[string]any{
				"OBJECTID":   3159367,
				"Incident":   "F26198635",
				"Nature":     "WF",
				"NatureDesc": "REPORTD WORKING FIRE     ",
				"Units":      "E2203:&#160;On&#160;Scene",
				"Channel":    "B3",
				"SymbolCode": "sc006-fire",
				"Date":       dateMillis,
				"GenLocInfo": "600 S COUNTRY CLUB DR ,MES",
			},
			"geometry": map[string]float64{"x": -111.84006, "y": 33.40284},
		}},
	}
	b, _ := json.Marshal(body)
	return string(b)
}

func zeroFeatureCanaryBody() string {
	body := map[string]any{
		"fields": []map[string]string{
			{"name": "OBJECTID"},
			{"name": "Incident"},
			{"name": "Nature"},
			{"name": "NatureDesc"},
			{"name": "Units"},
			{"name": "Channel"},
			{"name": "SymbolCode"},
			{"name": "Date"},
			{"name": "GenLocInfo"},
		},
		"spatialReference": map[string]int{"wkid": 4326},
		"features":         []map[string]any{},
	}
	b, _ := json.Marshal(body)
	return string(b)
}
