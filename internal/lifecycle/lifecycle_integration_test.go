package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func TestApplyWriteFailureDoesNotAdvanceLatestSuccess(t *testing.T) {
	pool := openLifecycleIntegrationPool(t)
	ctx := context.Background()
	st := store.NewWithPool(pool)
	manager := New(st, 2, slog.New(slog.NewTextHandler(io.Discard, nil)))
	source := "test-lifecycle-write-failure"
	older := time.Date(2026, 5, 28, 7, 0, 0, 0, time.UTC)
	newer := older.Add(5 * time.Minute)

	if err := manager.Apply(ctx, model.PollResult{
		Source:        source,
		RequestURL:    "https://example.test/ok",
		StartedAt:     older,
		FinishedAt:    older.Add(time.Second),
		StatusCode:    200,
		ParserVersion: "test-parser",
		Incidents: []model.Incident{{
			Source:       source,
			IncidentID:   "existing",
			NatureCode:   "MED",
			NatureDesc:   "Medical",
			LocationText: "1 N Test St",
			Lon:          -112.074,
			Lat:          33.4484,
			IncidentDate: older,
		}},
	}); err != nil {
		t.Fatalf("seed apply: %v", err)
	}

	err := manager.Apply(ctx, model.PollResult{
		Source:        source,
		RequestURL:    "https://example.test/bad",
		StartedAt:     newer,
		FinishedAt:    newer.Add(time.Second),
		StatusCode:    200,
		ParserVersion: "test-parser",
		Incidents: []model.Incident{{
			Source:       source,
			IncidentID:   "bad-json",
			NatureCode:   "MED",
			NatureDesc:   "Medical",
			LocationText: "2 N Test St",
			Lon:          -112.074,
			Lat:          33.4484,
			IncidentDate: newer,
			Raw:          []byte("{"),
		}},
	})
	if err == nil {
		t.Fatal("Apply succeeded after incident write failure")
	}

	latest, err := st.LatestSuccessAt(ctx, source)
	if err != nil {
		t.Fatal(err)
	}
	if !latest.Equal(older) {
		t.Fatalf("latest success = %s, want %s", latest, older)
	}

	var missingSince *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT missing_since
		FROM incidents
		WHERE source = $1 AND incident_id = 'existing'`, source,
	).Scan(&missingSince); err != nil {
		t.Fatal(err)
	}
	if missingSince != nil {
		t.Fatalf("existing incident was marked missing at %s after failed apply", missingSince.UTC())
	}
}

func TestOutageAndRecoveryOnlyCountAuthoritativeMisses(t *testing.T) {
	pool := openLifecycleIntegrationPool(t)
	ctx := context.Background()
	manager := New(store.NewWithPool(pool), 5, slog.New(slog.NewTextHandler(io.Discard, nil)))
	sourceName := "test-outage-recovery"
	start := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)

	incident := func(id string) model.Incident {
		return model.Incident{
			Source: sourceName, IncidentID: id, NatureCode: "WF", NatureDesc: "Working fire",
			Units: []model.Unit{}, Lon: -112.074, Lat: 33.4484, IncidentDate: start,
		}
	}
	apply := func(at time.Time, class model.PollClassification, reason string, incidents ...model.Incident) {
		t.Helper()
		result := model.PollResult{
			Source: sourceName, RequestURL: "https://example.test", StartedAt: at, FinishedAt: at.Add(time.Second),
			StatusCode: 200, ParserVersion: "test", Classification: class, Reason: reason, Incidents: incidents,
		}
		if class == model.PollFailure {
			result.StatusCode = 503
			result.Err = errors.New("upstream unavailable")
		}
		if err := manager.Apply(ctx, result); err != nil {
			t.Fatalf("apply %s at %s: %v", class, at, err)
		}
	}

	apply(start, model.PollAuthoritativeSuccess, "healthy", incident("A"), incident("B"), incident("C"))
	apply(start.Add(time.Minute), model.PollFailure, "recent_failures")
	apply(start.Add(2*time.Minute), model.PollFailure, "recent_failures")
	apply(start.Add(3*time.Minute), model.PollDegradedSnapshot, "payload_frozen", incident("A"))

	for _, id := range []string{"A", "B", "C"} {
		var lastSeen time.Time
		var missingSince *time.Time
		if err := pool.QueryRow(ctx, `SELECT last_seen_at, missing_since FROM incidents WHERE source=$1 AND incident_id=$2`, sourceName, id).Scan(&lastSeen, &missingSince); err != nil {
			t.Fatal(err)
		}
		if !lastSeen.Equal(start) || missingSince != nil {
			t.Fatalf("%s changed during outage: last_seen=%s missing_since=%v", id, lastSeen, missingSince)
		}
	}

	for miss := 1; miss <= 5; miss++ {
		apply(start.Add(time.Duration(3+miss)*time.Minute), model.PollAuthoritativeSuccess, "healthy", incident("A"), incident("D"))
	}

	for _, id := range []string{"B", "C"} {
		var clearedAt *time.Time
		if err := pool.QueryRow(ctx, `SELECT cleared_at FROM incidents WHERE source=$1 AND incident_id=$2`, sourceName, id).Scan(&clearedAt); err != nil {
			t.Fatal(err)
		}
		if clearedAt == nil {
			t.Fatalf("%s remained active after five authoritative recovery misses", id)
		}
	}
	var reopenEvents, dCreated int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incident_events WHERE source=$1 AND event_type='reopened'`, sourceName).Scan(&reopenEvents); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM incident_events WHERE source=$1 AND incident_id='D' AND event_type='created'`, sourceName).Scan(&dCreated); err != nil {
		t.Fatal(err)
	}
	if reopenEvents != 0 || dCreated != 1 {
		t.Fatalf("reopen events=%d, D created events=%d", reopenEvents, dCreated)
	}
}

func openLifecycleIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PHOENIX_FEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHOENIX_FEED_TEST_DATABASE_URL to run Postgres lifecycle integration tests")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("test_lifecycle_%d", time.Now().UnixNano())
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

	applyLifecycleSQLFile(t, pool, "../../db/schema.sql")
	return pool
}

func applyLifecycleSQLFile(t *testing.T, pool *pgxpool.Pool, path string) {
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
