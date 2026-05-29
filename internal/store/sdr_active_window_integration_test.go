package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestClearStaleSDRAudioIncidentsClearsExpiredOnly(t *testing.T) {
	pool := openStoreIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)

	seedIncident(t, pool, "sdr_audio", "sdr-old", now.Add(-2*time.Hour))
	seedIncident(t, pool, "sdr_audio", "sdr-fresh", now.Add(-10*time.Minute))
	seedIncident(t, pool, "phoenix-fire-mapserver", "map-old", now.Add(-2*time.Hour))

	st := NewWithPool(pool)
	cleared, err := st.ClearStaleSDRAudioIncidents(ctx, now, 90*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if cleared != 1 {
		t.Fatalf("cleared = %d, want 1", cleared)
	}

	assertClearedAt(t, pool, "sdr_audio", "sdr-old", true)
	assertClearedAt(t, pool, "sdr_audio", "sdr-fresh", false)
	assertClearedAt(t, pool, "phoenix-fire-mapserver", "map-old", false)

	var activeOld int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM active_incidents
		WHERE source = 'sdr_audio' AND incident_id = 'sdr-old'`).Scan(&activeOld); err != nil {
		t.Fatal(err)
	}
	if activeOld != 0 {
		t.Fatalf("cleared SDR active view count = %d, want 0", activeOld)
	}

	var activeFresh int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM active_incidents
		WHERE source = 'sdr_audio' AND incident_id = 'sdr-fresh'`).Scan(&activeFresh); err != nil {
		t.Fatal(err)
	}
	if activeFresh != 1 {
		t.Fatalf("fresh SDR active view count = %d, want 1", activeFresh)
	}
}

func TestClearStaleSDRAudioMigration(t *testing.T) {
	pool := openStoreIntegrationPoolWithoutMigrations(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO incidents (source, incident_id, nature_desc, incident_date, received_at, last_seen_at, geom)
		VALUES
			('sdr_audio', 'sdr-old', 'Overdose', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', ST_SetSRID(ST_MakePoint(-112.074, 33.4484), 4326)),
			('sdr_audio', 'sdr-fresh', 'Overdose', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', NOW() - INTERVAL '10 minutes', ST_SetSRID(ST_MakePoint(-112.074, 33.4484), 4326)),
			('phoenix-fire-mapserver', 'map-old', 'Overdose', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', NOW() - INTERVAL '3 hours', ST_SetSRID(ST_MakePoint(-112.074, 33.4484), 4326))`)
	if err != nil {
		t.Fatal(err)
	}

	applyStoreSQLFile(t, pool, "../../db/migrations/0006_clear_stale_sdr_audio_incidents.sql")
	firstCleared := getClearedAt(t, pool, "sdr_audio", "sdr-old")
	if firstCleared == nil {
		t.Fatal("stale SDR row was not cleared by migration")
	}
	assertClearedAt(t, pool, "sdr_audio", "sdr-fresh", false)
	assertClearedAt(t, pool, "phoenix-fire-mapserver", "map-old", false)

	applyStoreSQLFile(t, pool, "../../db/migrations/0006_clear_stale_sdr_audio_incidents.sql")
	secondCleared := getClearedAt(t, pool, "sdr_audio", "sdr-old")
	if secondCleared == nil || !secondCleared.Equal(*firstCleared) {
		t.Fatalf("migration is not idempotent: first=%v second=%v", firstCleared, secondCleared)
	}
}

func seedIncident(t *testing.T, pool *pgxpool.Pool, source, incidentID string, incidentDate time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO incidents (source, incident_id, nature_desc, incident_date, received_at, last_seen_at, geom)
		VALUES ($1,$2,'Overdose',$3,$3,$3,ST_SetSRID(ST_MakePoint(-112.074, 33.4484), 4326))`,
		source, incidentID, incidentDate,
	)
	if err != nil {
		t.Fatal(err)
	}
}

func assertClearedAt(t *testing.T, pool *pgxpool.Pool, source, incidentID string, wantCleared bool) {
	t.Helper()
	clearedAt := getClearedAt(t, pool, source, incidentID)
	if wantCleared && clearedAt == nil {
		t.Fatalf("%s/%s cleared_at is nil, want set", source, incidentID)
	}
	if !wantCleared && clearedAt != nil {
		t.Fatalf("%s/%s cleared_at = %s, want nil", source, incidentID, clearedAt)
	}
}

func getClearedAt(t *testing.T, pool *pgxpool.Pool, source, incidentID string) *time.Time {
	t.Helper()
	var clearedAt *time.Time
	if err := pool.QueryRow(context.Background(), `
		SELECT cleared_at
		FROM incidents
		WHERE source = $1 AND incident_id = $2`, source, incidentID,
	).Scan(&clearedAt); err != nil {
		t.Fatal(err)
	}
	return clearedAt
}

func openStoreIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := openStoreIntegrationPoolWithoutMigrations(t)
	for _, path := range []string{
		"../../db/migrations/0003_dispatch_transcripts.sql",
		"../../db/migrations/0004_incidents_id_and_geocode_cache.sql",
		"../../db/migrations/0005_cleanup_bad_natures.sql",
		"../../db/migrations/0006_clear_stale_sdr_audio_incidents.sql",
	} {
		applyStoreSQLFile(t, pool, path)
	}
	return pool
}

func openStoreIntegrationPoolWithoutMigrations(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PHOENIX_FEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHOENIX_FEED_TEST_DATABASE_URL to run Postgres store integration tests")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("test_store_%d", time.Now().UnixNano())
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

	applyStoreSQLFile(t, pool, "../../db/schema.sql")
	return pool
}

func applyStoreSQLFile(t *testing.T, pool *pgxpool.Pool, path string) {
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
