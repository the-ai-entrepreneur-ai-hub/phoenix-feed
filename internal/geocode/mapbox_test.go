package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestMapboxClientBuildsPhoenixBiasedRequest(t *testing.T) {
	var capturedPath string
	var capturedQuery url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.EscapedPath()
		capturedQuery = r.URL.Query()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"features": []map[string]any{
				{"center": []float64{-112.074, 33.4484}},
			},
		})
	}))
	defer server.Close()

	client, err := NewMapboxClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithLimiter(NoopLimiter{}))
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.Geocode(context.Background(), "2350 West Obispo Avenue")
	if err != nil {
		t.Fatal(err)
	}

	if got.Lon != -112.074 || got.Lat != 33.4484 {
		t.Fatalf("result = %+v", got)
	}
	if capturedPath != "/geocoding/v5/mapbox.places/2350%20West%20Obispo%20Avenue%2C%20Phoenix%2C%20AZ.json" {
		t.Fatalf("path = %q", capturedPath)
	}
	assertQuery(t, capturedQuery, "access_token", "test-token")
	assertQuery(t, capturedQuery, "proximity", "-112.0740,33.4484")
	assertQuery(t, capturedQuery, "bbox", "-112.5,33.0,-111.5,33.9")
	assertQuery(t, capturedQuery, "country", "us")
	assertQuery(t, capturedQuery, "types", "address,place")
	assertQuery(t, capturedQuery, "limit", "1")
}

func TestMapboxClientReturnsNoResultOnEmptyFeatures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"features": []any{}})
	}))
	defer server.Close()

	client, err := NewMapboxClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithLimiter(NoopLimiter{}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Geocode(context.Background(), "Not A Real Address")
	if err != ErrNoResult {
		t.Fatalf("err = %v, want ErrNoResult", err)
	}
}

func TestMapboxClientTreatsBadRequestAsPermanentFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	client, err := NewMapboxClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithLimiter(NoopLimiter{}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Geocode(context.Background(), "Not A Real Address")
	if !errors.Is(err, ErrNoResult) {
		t.Fatalf("err = %v, want ErrNoResult", err)
	}
	if !IsPermanentFailure(err) {
		t.Fatalf("err = %v, want permanent failure", err)
	}
}

func TestMapboxClientTreatsRateLimitAsRetryableFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := NewMapboxClient("test-token", WithBaseURL(server.URL), WithHTTPClient(server.Client()), WithLimiter(NoopLimiter{}))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Geocode(context.Background(), "2350 West Obispo Avenue")
	if err == nil {
		t.Fatal("Geocode succeeded, want rate limit error")
	}
	if IsPermanentFailure(err) {
		t.Fatalf("err = %v, want retryable failure", err)
	}
}

func TestMapboxClientRejectsMissingToken(t *testing.T) {
	if _, err := NewMapboxClient("", WithLimiter(NoopLimiter{})); err == nil {
		t.Fatal("NewMapboxClient without token succeeded")
	}
}

func assertQuery(t *testing.T, values url.Values, key, want string) {
	t.Helper()
	if got := values.Get(key); got != want {
		t.Fatalf("query %s = %q, want %q", key, got, want)
	}
}
