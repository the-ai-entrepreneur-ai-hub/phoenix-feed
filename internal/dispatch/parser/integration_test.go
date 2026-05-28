package dispatchparser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/api"
	"github.com/abusedmindset/phoenix-feed/internal/geocode"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestProcessBatchPromotesOnlyCanonicalRows(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := time.Date(2026, 5, 28, 7, 10, 0, 0, time.UTC)
	samples := []struct {
		text       string
		confidence float64
	}{
		{"Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92},
		{"AMR 207. CDEC 12. Fall. CLF. 154-35 East Cabern Drive.", 0.91},
		{"Medic 257 CDEC 4, CDEC 4, cardiac problem. West Cypress Street and South Gilbert Road.", 0.88},
		{"Engine 205 and Medic 210 CDEC 3 cardiac problems 925 South 35th Place.", 0.9},
		{"", 0.99},
		{"The point spread moved by three and a half before the closing bell.", 0.95},
		{"31 minutes on the timer, copy the update from command.", 0.93},
		{"Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue.", 0.79},
	}
	for i, sample := range samples {
		insertTranscript(t, pool, fmt.Sprintf("sample_%02d.wav", i), capturedAt.Add(time.Duration(i)*time.Second), sample.text, sample.confidence)
	}

	worker := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})
	stats, err := worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}

	if stats.BatchSize != 8 || stats.GatePassCount != 4 || stats.GateFailCount != 4 || stats.IncidentsInserted != 4 {
		t.Fatalf("stats = %+v", stats)
	}
	var incidentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE source = 'sdr_audio'`).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if incidentCount != 4 {
		t.Fatalf("incident count = %d, want 4", incidentCount)
	}
}

func TestProcessBatchSurfacesSDRIncidentFromActiveEndpoint(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := time.Date(2026, 5, 28, 7, 10, 0, 0, time.UTC)
	insertTranscript(t, pool, "engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	worker := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})
	if _, err := worker.ProcessBatch(ctx, 50); err != nil {
		t.Fatal(err)
	}

	st := store.NewWithPool(pool)
	req := httptest.NewRequest(http.MethodGet, "/v1/incidents/active", nil)
	rr := httptest.NewRecorder()
	api.Router(st, api.Config{Now: fixedNow}, slog.Default()).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Incidents []struct {
			Source       string  `json:"source"`
			IncidentID   string  `json:"incident_id"`
			LocationText string  `json:"location_text"`
			Lon          float64 `json:"lon"`
			Lat          float64 `json:"lat"`
		} `json:"incidents"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Incidents) != 1 {
		t.Fatalf("incidents = %+v, want one SDR row", body.Incidents)
	}
	got := body.Incidents[0]
	if got.Source != SourceName || got.IncidentID != "sdr-1" || got.LocationText != "2350 West Obispo Avenue" {
		t.Fatalf("incident = %+v", got)
	}
	if got.Lon != -112.074 || got.Lat != 33.4484 {
		t.Fatalf("coordinates = %f,%f", got.Lon, got.Lat)
	}
}

func TestTwoWorkersDoNotDoubleInsert(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := time.Date(2026, 5, 28, 7, 10, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		insertTranscript(t, pool, fmt.Sprintf("race_%02d.wav", i), capturedAt.Add(time.Duration(i)*time.Second), "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)
	}
	workerA := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})
	workerB := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, worker := range []*Worker{workerA, workerB} {
		wg.Add(1)
		go func(w *Worker) {
			defer wg.Done()
			_, err := w.ProcessBatch(ctx, 50)
			errs <- err
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	var count, distinctCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*), COUNT(DISTINCT incident_id) FROM incidents WHERE source = 'sdr_audio'`).Scan(&count, &distinctCount); err != nil {
		t.Fatal(err)
	}
	if count != 20 || distinctCount != 20 {
		t.Fatalf("count=%d distinct=%d, want 20/20", count, distinctCount)
	}
}

type stubGeocoder struct{}

func (stubGeocoder) Geocode(context.Context, string) (geocode.Result, error) {
	return geocode.Result{Lon: -112.074, Lat: 33.4484}, nil
}

func fixedNow() time.Time {
	return time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
}

func insertTranscript(t *testing.T, pool *pgxpool.Pool, wav string, capturedAt time.Time, text string, confidence float64) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO dispatch_transcripts (
			wav_filename, captured_at, display_text, verification_confidence, raw_payload
		) VALUES ($1,$2,$3,$4,$5::jsonb)`,
		wav, capturedAt, text, confidence, `{"test":true}`,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func openIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PHOENIX_FEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHOENIX_FEED_TEST_DATABASE_URL to run Postgres parser integration tests")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = adminPool.Exec(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	for _, path := range []string{
		"../../../db/schema.sql",
		"../../../db/migrations/0003_dispatch_transcripts.sql",
		"../../../db/migrations/0004_incidents_id_and_geocode_cache.sql",
	} {
		applySQLFile(t, pool, path)
	}
	return pool
}

func applySQLFile(t *testing.T, pool *pgxpool.Pool, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range strings.Split(string(body), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			t.Fatalf("apply %s statement %q: %v", path, stmt, err)
		}
	}
}
