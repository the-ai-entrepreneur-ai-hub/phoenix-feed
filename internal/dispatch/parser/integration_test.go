package dispatchparser

import (
	"context"
	"database/sql"
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

func TestProcessBatchRejectsStaleCapture(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := fixedNow().Add(-3 * time.Hour)
	insertTranscript(t, pool, "stale_engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	worker := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})
	stats, err := worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}

	if stats.BatchSize != 1 || stats.GateFailCount != 1 || stats.StaleCaptureCount != 1 || stats.IncidentsInserted != 0 {
		t.Fatalf("stats = %+v", stats)
	}
	var parsedAt *time.Time
	var parsedIncidentID *int64
	if err := pool.QueryRow(ctx, `
		SELECT parsed_at, parsed_incident_id
		FROM dispatch_transcripts
		WHERE wav_filename = 'stale_engine_2510.wav'`).Scan(&parsedAt, &parsedIncidentID); err != nil {
		t.Fatal(err)
	}
	if parsedAt == nil {
		t.Fatal("parsed_at is nil, want stale transcript consumed")
	}
	if parsedIncidentID != nil {
		t.Fatalf("parsed_incident_id = %d, want nil", *parsedIncidentID)
	}
	var incidentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE source = 'sdr_audio'`).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if incidentCount != 0 {
		t.Fatalf("incident count = %d, want 0", incidentCount)
	}
}

func TestProcessBatchPromotesFreshCapture(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := fixedNow().Add(-10 * time.Minute)
	insertTranscript(t, pool, "fresh_engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	worker := NewWorker(pool, stubGeocoder{}, slog.Default(), WorkerOptions{Now: fixedNow})
	stats, err := worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}

	if stats.BatchSize != 1 || stats.GatePassCount != 1 || stats.StaleCaptureCount != 0 || stats.IncidentsInserted != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	var incidentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE source = 'sdr_audio' AND cleared_at IS NULL`).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if incidentCount != 1 {
		t.Fatalf("active incident count = %d, want 1", incidentCount)
	}
}

func TestProcessBatchRetriesTransientGeocodeFailure(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := fixedNow().Add(-10 * time.Minute)
	insertTranscript(t, pool, "retry_engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	geocoder := &sequenceGeocoder{
		errs: []error{context.DeadlineExceeded},
		results: []geocode.Result{
			{Lon: -112.074, Lat: 33.4484},
		},
	}
	worker := NewWorker(pool, geocoder, slog.Default(), WorkerOptions{Now: fixedNow})
	stats, err := worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.BatchSize != 1 || stats.GeocodeFailCount != 1 || stats.IncidentsInserted != 0 {
		t.Fatalf("stats after timeout = %+v", stats)
	}

	var parsedAt sql.NullTime
	var attempts sql.NullInt64
	if err := pool.QueryRow(ctx, `
		SELECT parsed_at, geocode_attempts
		FROM dispatch_transcripts
		WHERE wav_filename = 'retry_engine_2510.wav'`).Scan(&parsedAt, &attempts); err != nil {
		t.Fatal(err)
	}
	if parsedAt.Valid {
		t.Fatalf("parsed_at = %s, want null after transient geocode failure", parsedAt.Time)
	}
	if !attempts.Valid || attempts.Int64 != 1 {
		t.Fatalf("geocode_attempts = %+v, want 1", attempts)
	}
	if geocoder.calls != 1 {
		t.Fatalf("geocoder calls = %d, want 1", geocoder.calls)
	}

	stats, err = worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.BatchSize != 1 || stats.GeocodePassCount != 1 || stats.IncidentsInserted != 1 {
		t.Fatalf("stats after retry = %+v", stats)
	}
	if err := pool.QueryRow(ctx, `
		SELECT parsed_at, parsed_incident_id
		FROM dispatch_transcripts
		WHERE wav_filename = 'retry_engine_2510.wav'`).Scan(&parsedAt, new(sql.NullInt64)); err != nil {
		t.Fatal(err)
	}
	if !parsedAt.Valid {
		t.Fatal("parsed_at stayed null after successful retry")
	}
}

func TestProcessBatchConsumesPermanentGeocodeNoResult(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := fixedNow().Add(-10 * time.Minute)
	insertTranscript(t, pool, "no_result_engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	worker := NewWorker(pool, &sequenceGeocoder{errs: []error{geocode.ErrNoResult}}, slog.Default(), WorkerOptions{Now: fixedNow})
	stats, err := worker.ProcessBatch(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if stats.BatchSize != 1 || stats.GeocodeFailCount != 1 || stats.IncidentsInserted != 0 {
		t.Fatalf("stats = %+v", stats)
	}

	var parsedAt sql.NullTime
	var parsedIncidentID sql.NullInt64
	if err := pool.QueryRow(ctx, `
		SELECT parsed_at, parsed_incident_id
		FROM dispatch_transcripts
		WHERE wav_filename = 'no_result_engine_2510.wav'`).Scan(&parsedAt, &parsedIncidentID); err != nil {
		t.Fatal(err)
	}
	if !parsedAt.Valid {
		t.Fatal("parsed_at is null, want permanent geocode failure consumed")
	}
	if parsedIncidentID.Valid {
		t.Fatalf("parsed_incident_id = %d, want null", parsedIncidentID.Int64)
	}
	var incidentCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents WHERE source = 'sdr_audio'`).Scan(&incidentCount); err != nil {
		t.Fatal(err)
	}
	if incidentCount != 0 {
		t.Fatalf("incident count = %d, want 0", incidentCount)
	}
}

func TestProcessBatchConsumesRetryableGeocodeFailureAfterDefaultAttemptCap(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	capturedAt := fixedNow().Add(-10 * time.Minute)
	insertTranscript(t, pool, "retry_cap_engine_2510.wav", capturedAt, "Engine 2510. CDEC 4. Overdose. 2350 West Obispo Avenue. Unit 204.", 0.92)

	worker := NewWorker(pool, &sequenceGeocoder{err: context.DeadlineExceeded}, slog.Default(), WorkerOptions{Now: fixedNow})
	for attempt := 1; attempt <= 4; attempt++ {
		if _, err := worker.ProcessBatch(ctx, 50); err != nil {
			t.Fatal(err)
		}
		var parsedAt sql.NullTime
		var attempts sql.NullInt64
		if err := pool.QueryRow(ctx, `
			SELECT parsed_at, geocode_attempts
			FROM dispatch_transcripts
			WHERE wav_filename = 'retry_cap_engine_2510.wav'`).Scan(&parsedAt, &attempts); err != nil {
			t.Fatal(err)
		}
		if parsedAt.Valid {
			t.Fatalf("attempt %d parsed_at = %s, want null before cap", attempt, parsedAt.Time)
		}
		if !attempts.Valid || attempts.Int64 != int64(attempt) {
			t.Fatalf("attempt %d geocode_attempts = %+v", attempt, attempts)
		}
	}

	if _, err := worker.ProcessBatch(ctx, 50); err != nil {
		t.Fatal(err)
	}
	var parsedAt sql.NullTime
	var attempts sql.NullInt64
	if err := pool.QueryRow(ctx, `
		SELECT parsed_at, geocode_attempts
		FROM dispatch_transcripts
		WHERE wav_filename = 'retry_cap_engine_2510.wav'`).Scan(&parsedAt, &attempts); err != nil {
		t.Fatal(err)
	}
	if !parsedAt.Valid {
		t.Fatal("parsed_at is null, want consumed at default retry cap")
	}
	if !attempts.Valid || attempts.Int64 != 5 {
		t.Fatalf("geocode_attempts = %+v, want 5", attempts)
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

func TestCleanupBadNaturesMigration(t *testing.T) {
	pool := openIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)

	_, err := pool.Exec(ctx, `
		INSERT INTO incidents (source, incident_id, nature_desc, incident_date, received_at, last_seen_at)
		VALUES
			('sdr_audio', 'sdr-bad', 'Difficulty Breathing, 447, East Broadway Ave, Unit 2, Ladder 263, Sea Deck 4, Difficulty Breathing', $1, $1, $1),
			('sdr_audio', 'sdr-no-comma', 'Long Nature Text Without A Comma That Should Stay Unchanged By The Cleanup', $1, $1, $1),
			('phoenix-fire-mapserver', 'map-bad', 'Difficulty Breathing, 447, East Broadway Ave, Unit 2', $1, $1, $1)`,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	applySQLFile(t, pool, "../../../db/migrations/0005_cleanup_bad_natures.sql")

	rows, err := pool.Query(ctx, `
		SELECT incident_id, nature_desc
		FROM incidents
		WHERE incident_id IN ('sdr-bad', 'sdr-no-comma', 'map-bad')`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var id, nature string
		if err := rows.Scan(&id, &nature); err != nil {
			t.Fatal(err)
		}
		got[id] = nature
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if got["sdr-bad"] != "Difficulty Breathing" {
		t.Fatalf("cleaned nature = %q, want Difficulty Breathing", got["sdr-bad"])
	}
	if got["sdr-no-comma"] != "Long Nature Text Without A Comma That Should Stay Unchanged By The Cleanup" {
		t.Fatalf("no-comma nature changed to %q", got["sdr-no-comma"])
	}
	if got["map-bad"] != "Difficulty Breathing, 447, East Broadway Ave, Unit 2" {
		t.Fatalf("mapserver nature changed to %q", got["map-bad"])
	}
}

type stubGeocoder struct{}

func (stubGeocoder) Geocode(context.Context, string) (geocode.Result, error) {
	return geocode.Result{Lon: -112.074, Lat: 33.4484}, nil
}

type sequenceGeocoder struct {
	err     error
	errs    []error
	results []geocode.Result
	calls   int
}

func (g *sequenceGeocoder) Geocode(context.Context, string) (geocode.Result, error) {
	g.calls++
	if len(g.errs) > 0 {
		err := g.errs[0]
		g.errs = g.errs[1:]
		if err != nil {
			return geocode.Result{}, err
		}
	}
	if g.err != nil {
		return geocode.Result{}, g.err
	}
	if len(g.results) > 0 {
		result := g.results[0]
		g.results = g.results[1:]
		return result, nil
	}
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
		"../../../db/migrations/0005_cleanup_bad_natures.sql",
		"../../../db/migrations/0006_clear_stale_sdr_audio_incidents.sql",
		"../../../db/migrations/0007_dispatch_transcript_geocode_attempts.sql",
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
