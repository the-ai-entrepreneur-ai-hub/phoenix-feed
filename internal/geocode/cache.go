package geocode

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CacheEntry struct {
	Result     Result
	Success    bool
	GeocodedAt time.Time
}

type Cache interface {
	Lookup(context.Context, string, time.Time) (CacheEntry, bool, error)
	Store(context.Context, string, Result, bool, time.Time) error
}

type CachedGeocoder struct {
	cache    Cache
	provider Provider
	now      func() time.Time
}

func NewCachedGeocoder(cache Cache, provider Provider, now func() time.Time) *CachedGeocoder {
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CachedGeocoder{cache: cache, provider: provider, now: now}
}

func (g *CachedGeocoder) Geocode(ctx context.Context, address string) (Result, error) {
	now := g.now().UTC()
	if g.cache != nil {
		entry, hit, err := g.cache.Lookup(ctx, address, now)
		if err != nil {
			return Result{}, err
		}
		if hit {
			if entry.Success {
				return entry.Result, nil
			}
			if now.Sub(entry.GeocodedAt) < failedCacheTTL {
				return Result{}, ErrNoResult
			}
		}
	}

	result, err := g.provider.Geocode(ctx, address)
	if err != nil {
		if g.cache != nil && IsPermanentFailure(err) {
			_ = g.cache.Store(ctx, address, Result{}, false, now)
		}
		return Result{}, err
	}
	if g.cache != nil {
		if err := g.cache.Store(ctx, address, result, true, now); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

type DB interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type PostgresCache struct {
	db DB
}

func NewPostgresCache(db DB) *PostgresCache {
	return &PostgresCache{db: db}
}

func (c *PostgresCache) Lookup(ctx context.Context, address string, now time.Time) (CacheEntry, bool, error) {
	var lon, lat sql.NullFloat64
	var geocodedAt time.Time
	var success bool
	err := c.db.QueryRow(ctx, `
		UPDATE geocode_cache
		SET hits = hits + 1
		WHERE address = $1
		RETURNING lon::float8, lat::float8, geocoded_at, success`,
		address,
	).Scan(&lon, &lat, &geocodedAt, &success)
	if err == pgx.ErrNoRows {
		return CacheEntry{}, false, nil
	}
	if err != nil {
		return CacheEntry{}, false, err
	}
	return CacheEntry{
		Result:     Result{Lon: lon.Float64, Lat: lat.Float64},
		Success:    success,
		GeocodedAt: geocodedAt.UTC(),
	}, true, nil
}

func (c *PostgresCache) Store(ctx context.Context, address string, result Result, success bool, now time.Time) error {
	var lon, lat any
	if success {
		lon = result.Lon
		lat = result.Lat
	}
	_, err := c.db.Exec(ctx, `
		INSERT INTO geocode_cache (address, lon, lat, geocoded_at, success)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (address) DO UPDATE SET
			lon = EXCLUDED.lon,
			lat = EXCLUDED.lat,
			geocoded_at = EXCLUDED.geocoded_at,
			success = EXCLUDED.success`,
		address, lon, lat, now.UTC(), success,
	)
	return err
}
