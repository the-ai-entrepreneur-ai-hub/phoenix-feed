package lifecycle

import (
	"context"
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
