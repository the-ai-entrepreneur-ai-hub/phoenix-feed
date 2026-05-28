package geocode

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresCacheHitMissAndFailureRows(t *testing.T) {
	pool := openGeocodeIntegrationPool(t)
	ctx := context.Background()
	cache := NewPostgresCache(pool)
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)

	if _, hit, err := cache.Lookup(ctx, "missing", now); err != nil || hit {
		t.Fatalf("missing lookup hit=%v err=%v, want miss", hit, err)
	}

	if err := cache.Store(ctx, "2350 West Obispo Avenue", Result{Lon: -112.074, Lat: 33.4484}, true, now); err != nil {
		t.Fatal(err)
	}
	entry, hit, err := cache.Lookup(ctx, "2350 West Obispo Avenue", now)
	if err != nil {
		t.Fatal(err)
	}
	if !hit || !entry.Success || entry.Result.Lon != -112.074 || entry.Result.Lat != 33.4484 {
		t.Fatalf("success entry=%+v hit=%v", entry, hit)
	}
	assertCacheHits(t, pool, "2350 West Obispo Avenue", 1)

	if err := cache.Store(ctx, "bad address", Result{}, false, now); err != nil {
		t.Fatal(err)
	}
	entry, hit, err = cache.Lookup(ctx, "bad address", now)
	if err != nil {
		t.Fatal(err)
	}
	if !hit || entry.Success {
		t.Fatalf("failure entry=%+v hit=%v", entry, hit)
	}
	assertCacheHits(t, pool, "bad address", 1)
}

func assertCacheHits(t *testing.T, pool *pgxpool.Pool, address string, want int) {
	t.Helper()
	var hits int
	if err := pool.QueryRow(context.Background(), `SELECT hits FROM geocode_cache WHERE address = $1`, address).Scan(&hits); err != nil {
		t.Fatal(err)
	}
	if hits != want {
		t.Fatalf("hits for %q = %d, want %d", address, hits, want)
	}
}

func openGeocodeIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("PHOENIX_FEED_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PHOENIX_FEED_TEST_DATABASE_URL to run Postgres geocode cache integration tests")
	}

	ctx := context.Background()
	schema := fmt.Sprintf("test_geocode_%d", time.Now().UnixNano())
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
		"../../db/schema.sql",
		"../../db/migrations/0004_incidents_id_and_geocode_cache.sql",
	} {
		applyGeocodeSQLFile(t, pool, path)
	}
	return pool
}

func applyGeocodeSQLFile(t *testing.T, pool *pgxpool.Pool, path string) {
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
