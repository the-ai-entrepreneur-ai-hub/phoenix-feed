package geocode

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCachedGeocoderReturnsSuccessfulCacheHitWithoutProviderCall(t *testing.T) {
	cache := newFakeCache()
	cache.rows["2350 West Obispo Avenue"] = cacheRow{
		result:     Result{Lon: -112.074, Lat: 33.4484},
		success:    true,
		geocodedAt: time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC),
	}
	provider := &fakeProvider{}
	geocoder := NewCachedGeocoder(cache, provider, func() time.Time {
		return time.Date(2026, 5, 29, 8, 0, 0, 0, time.UTC)
	})

	got, err := geocoder.Geocode(context.Background(), "2350 West Obispo Avenue")
	if err != nil {
		t.Fatal(err)
	}
	if got.Lon != -112.074 || got.Lat != 33.4484 {
		t.Fatalf("result = %+v", got)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if cache.rows["2350 West Obispo Avenue"].hits != 1 {
		t.Fatalf("hits = %d, want 1", cache.rows["2350 West Obispo Avenue"].hits)
	}
}

func TestCachedGeocoderReturnsFreshFailureCacheHit(t *testing.T) {
	cache := newFakeCache()
	cache.rows["bad"] = cacheRow{
		success:    false,
		geocodedAt: time.Date(2026, 5, 28, 7, 0, 0, 0, time.UTC),
	}
	provider := &fakeProvider{result: Result{Lon: 1, Lat: 2}}
	geocoder := NewCachedGeocoder(cache, provider, func() time.Time {
		return time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
	})

	_, err := geocoder.Geocode(context.Background(), "bad")
	if err != ErrNoResult {
		t.Fatalf("err = %v, want ErrNoResult", err)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
	if cache.rows["bad"].hits != 1 {
		t.Fatalf("hits = %d, want 1", cache.rows["bad"].hits)
	}
}

func TestCachedGeocoderRetriesStaleFailureAndStoresSuccess(t *testing.T) {
	cache := newFakeCache()
	cache.rows["stale"] = cacheRow{
		success:    false,
		geocodedAt: time.Date(2026, 5, 27, 7, 0, 0, 0, time.UTC),
	}
	provider := &fakeProvider{result: Result{Lon: -111.9, Lat: 33.5}}
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
	geocoder := NewCachedGeocoder(cache, provider, func() time.Time { return now })

	got, err := geocoder.Geocode(context.Background(), "stale")
	if err != nil {
		t.Fatal(err)
	}
	if got != provider.result {
		t.Fatalf("result = %+v, want %+v", got, provider.result)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	row := cache.rows["stale"]
	if !row.success || row.geocodedAt != now || row.result != provider.result {
		t.Fatalf("stored row = %+v", row)
	}
}

func TestCachedGeocoderStoresProviderFailure(t *testing.T) {
	cache := newFakeCache()
	provider := &fakeProvider{err: errProviderFailed}
	now := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
	geocoder := NewCachedGeocoder(cache, provider, func() time.Time { return now })

	_, err := geocoder.Geocode(context.Background(), "bad")
	if !errors.Is(err, errProviderFailed) {
		t.Fatalf("err = %v, want provider failure", err)
	}
	row := cache.rows["bad"]
	if row.success || row.geocodedAt != now {
		t.Fatalf("stored row = %+v", row)
	}
}

var errProviderFailed = errors.New("provider failed")

type fakeProvider struct {
	result Result
	err    error
	calls  int
}

func (f *fakeProvider) Geocode(context.Context, string) (Result, error) {
	f.calls++
	if f.err != nil {
		return Result{}, f.err
	}
	return f.result, nil
}

type fakeCache struct {
	rows map[string]cacheRow
}

type cacheRow struct {
	result     Result
	success    bool
	geocodedAt time.Time
	hits       int
}

func newFakeCache() *fakeCache {
	return &fakeCache{rows: map[string]cacheRow{}}
}

func (f *fakeCache) Lookup(ctx context.Context, address string, now time.Time) (CacheEntry, bool, error) {
	row, ok := f.rows[address]
	if !ok {
		return CacheEntry{}, false, nil
	}
	row.hits++
	f.rows[address] = row
	return CacheEntry{
		Result:     row.result,
		Success:    row.success,
		GeocodedAt: row.geocodedAt,
	}, true, nil
}

func (f *fakeCache) Store(ctx context.Context, address string, result Result, success bool, now time.Time) error {
	f.rows[address] = cacheRow{result: result, success: success, geocodedAt: now}
	return nil
}
