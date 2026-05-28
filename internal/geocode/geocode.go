// Package geocode wraps Phoenix-biased Mapbox address geocoding and cache
// lookup/storage for SDR-derived incidents.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
)

const (
	defaultMapboxBaseURL = "https://api.mapbox.com"
	phoenixSuffix        = ", Phoenix, AZ"
	failedCacheTTL       = 24 * time.Hour
)

var ErrNoResult = errors.New("geocode no result")

type Result struct {
	Lon float64
	Lat float64
}

type Provider interface {
	Geocode(context.Context, string) (Result, error)
}

type Waiter interface {
	Wait(context.Context) error
}

type NoopLimiter struct{}

func (NoopLimiter) Wait(context.Context) error { return nil }

type MapboxClient struct {
	token      string
	baseURL    string
	httpClient *http.Client
	limiter    Waiter
}

type MapboxOption func(*MapboxClient)

func WithBaseURL(baseURL string) MapboxOption {
	return func(c *MapboxClient) {
		c.baseURL = strings.TrimRight(baseURL, "/")
	}
}

func WithHTTPClient(client *http.Client) MapboxOption {
	return func(c *MapboxClient) {
		if client != nil {
			c.httpClient = client
		}
	}
}

func WithLimiter(limiter Waiter) MapboxOption {
	return func(c *MapboxClient) {
		if limiter != nil {
			c.limiter = limiter
		}
	}
}

func NewMapboxClient(token string, opts ...MapboxOption) (*MapboxClient, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("MAPBOX_TOKEN is required for dispatch parser geocoding")
	}
	c := &MapboxClient{
		token:      token,
		baseURL:    defaultMapboxBaseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		limiter:    ratelimit.NewTokenBucket(time.Second/5, 5),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

func (c *MapboxClient) Geocode(ctx context.Context, address string) (Result, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return Result{}, ErrNoResult
	}
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return Result{}, err
		}
	}

	queryAddress := address
	if !strings.Contains(strings.ToLower(queryAddress), "phoenix") {
		queryAddress += phoenixSuffix
	}
	u, err := url.Parse(c.baseURL + "/geocoding/v5/mapbox.places/" + url.PathEscape(queryAddress) + ".json")
	if err != nil {
		return Result{}, fmt.Errorf("mapbox url: %w", err)
	}
	q := u.Query()
	q.Set("access_token", c.token)
	q.Set("proximity", "-112.0740,33.4484")
	q.Set("bbox", "-112.5,33.0,-111.5,33.9")
	q.Set("country", "us")
	q.Set("types", "address,place")
	q.Set("limit", "1")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Result{}, fmt.Errorf("mapbox request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("mapbox http: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("mapbox status %d", resp.StatusCode)
	}

	var body struct {
		Features []struct {
			Center []float64 `json:"center"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Result{}, fmt.Errorf("mapbox decode: %w", err)
	}
	if len(body.Features) == 0 || len(body.Features[0].Center) < 2 {
		return Result{}, ErrNoResult
	}
	return Result{Lon: body.Features[0].Center[0], Lat: body.Features[0].Center[1]}, nil
}
