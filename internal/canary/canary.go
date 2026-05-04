// Package canary verifies upstream source contracts before drift reaches users.
package canary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const defaultHTTPTimeout = 20 * time.Second

type Store interface {
	FeatureCountBaseline(context.Context, string, time.Time) (float64, bool, error)
	RecordCanary(context.Context, store.ContractCanaryResult) error
}

type Checker struct {
	URL    string
	Store  Store
	Client *http.Client
	Now    func() time.Time
	Log    *slog.Logger
}

type rawArcGIS struct {
	Fields []struct {
		Name string `json:"name"`
	} `json:"fields"`
	SpatialReference struct {
		WKID       int `json:"wkid"`
		LatestWKID int `json:"latestWkid"`
	} `json:"spatialReference"`
	Features []struct {
		Attributes map[string]any `json:"attributes"`
		Geometry   struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"geometry"`
	} `json:"features"`
}

// NewPhoenixFire returns a checker for the production Phoenix Fire MapServer.
func NewPhoenixFire(st Store, log *slog.Logger) Checker {
	return Checker{
		URL:    phxfire.DefaultQueryURL(),
		Store:  st,
		Client: &http.Client{Timeout: defaultHTTPTimeout},
		Log:    log,
	}
}

// Run performs a canary check and writes it to contract_canary.
func (c Checker) Run(ctx context.Context) (store.ContractCanaryResult, error) {
	result, err := c.Check(ctx)
	if err != nil {
		return result, err
	}
	if c.Store != nil {
		if err := c.Store.RecordCanary(ctx, result); err != nil {
			return result, err
		}
	}
	if !result.Passed && c.Log != nil {
		c.Log.Error("contract canary failed", "source", result.Source, "drift", string(result.Drift))
	}
	return result, nil
}

// Check performs the six architecture.md section 7 assertions.
func (c Checker) Check(ctx context.Context) (store.ContractCanaryResult, error) {
	now := c.now()
	expected := phxfire.ExpectedFields()
	result := store.ContractCanaryResult{
		Source:         phxfire.SourceName,
		CheckedAt:      now,
		ExpectedFields: expected,
		Passed:         true,
		ParserVersion:  phxfire.ParserVersion,
	}

	drift := map[string]any{}
	fail := func(key string, value any) {
		result.Passed = false
		drift[key] = value
	}
	note := func(key string, value any) {
		drift[key] = value
	}

	body, status, err := c.fetch(ctx)
	if err != nil {
		fail("http_error", err.Error())
		result.Drift = encodeDrift(drift)
		return result, nil
	}
	if status != http.StatusOK {
		fail("http_status", status)
		result.Drift = encodeDrift(drift)
		return result, nil
	}

	var raw rawArcGIS
	if err := json.Unmarshal(body, &raw); err != nil {
		fail("json_parse", err.Error())
		result.Drift = encodeDrift(drift)
		return result, nil
	}

	actualFields := actualFieldNames(raw)
	result.ActualFields = actualFields
	if missing := missingFields(expected, actualFields); len(missing) > 0 {
		fail("missing_fields", missing)
	}

	outputSR := outputSR(raw)
	if outputSR != 0 {
		result.OutputSR = &outputSR
	}

	result.FeatureCount = len(raw.Features)
	result.GeomPlausible = geometryPlausible(raw)
	if result.FeatureCount > 0 && !result.GeomPlausible {
		fail("geometry", "coordinates are not plausible WGS84 lon/lat")
	}
	if outputSR != 0 && outputSR != 4326 {
		fail("output_sr", outputSR)
	}

	c.checkBaseline(ctx, now, result.FeatureCount, fail, note)
	checkDateWindow(raw, now, fail, note)
	if _, err := phxfire.ParseFeatures(body); err != nil {
		fail("production_parser", err.Error())
	}

	result.Drift = encodeDrift(drift)
	return result, nil
}

func (c Checker) fetch(ctx context.Context) ([]byte, int, error) {
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	url := c.URL
	if url == "" {
		url = phxfire.DefaultQueryURL()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "phoenix-feed-canary/0.2")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return body, resp.StatusCode, nil
}

func (c Checker) checkBaseline(ctx context.Context, now time.Time, featureCount int, fail func(string, any), note func(string, any)) {
	if c.Store == nil {
		note("baseline_unavailable", true)
		return
	}
	avg, ok, err := c.Store.FeatureCountBaseline(ctx, phxfire.SourceName, now.Add(-7*24*time.Hour))
	if err != nil {
		fail("baseline_error", err.Error())
		return
	}
	if !ok {
		note("baseline_unavailable", true)
		return
	}
	if avg > 0 && featureCount == 0 {
		fail("feature_count", map[string]any{"actual": featureCount, "baseline_avg": avg})
		return
	}
	if avg > 0 && float64(featureCount) > avg*10 {
		fail("feature_count", map[string]any{"actual": featureCount, "baseline_avg": avg})
	}
}

func actualFieldNames(raw rawArcGIS) []string {
	fields := map[string]struct{}{}
	for _, field := range raw.Fields {
		if field.Name != "" {
			fields[field.Name] = struct{}{}
		}
	}
	if len(fields) == 0 && len(raw.Features) > 0 {
		for name := range raw.Features[0].Attributes {
			fields[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	sort.Strings(out)
	return out
}

func missingFields(expected, actual []string) []string {
	actualSet := map[string]struct{}{}
	for _, field := range actual {
		actualSet[field] = struct{}{}
	}
	missing := []string{}
	for _, field := range expected {
		if _, ok := actualSet[field]; !ok {
			missing = append(missing, field)
		}
	}
	return missing
}

func outputSR(raw rawArcGIS) int {
	if raw.SpatialReference.LatestWKID != 0 {
		return raw.SpatialReference.LatestWKID
	}
	return raw.SpatialReference.WKID
}

func geometryPlausible(raw rawArcGIS) bool {
	if len(raw.Features) == 0 {
		return true
	}
	for _, feature := range raw.Features {
		x := feature.Geometry.X
		y := feature.Geometry.Y
		if x < -180 || x > 180 || y < -90 || y > 90 || (x == 0 && y == 0) {
			return false
		}
	}
	return true
}

func checkDateWindow(raw rawArcGIS, now time.Time, fail func(string, any), note func(string, any)) {
	if len(raw.Features) == 0 {
		note("date_sample_unavailable", true)
		return
	}
	rawDate, ok := raw.Features[0].Attributes["Date"]
	if !ok {
		fail("date_missing", true)
		return
	}
	millis, ok := numericMillis(rawDate)
	if !ok {
		fail("date_parse", rawDate)
		return
	}
	t := time.UnixMilli(millis).UTC()
	if t.Before(now.Add(-72*time.Hour)) || t.After(now.Add(10*time.Minute)) {
		fail("date_window", t.Format(time.RFC3339))
	}
}

func numericMillis(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func encodeDrift(drift map[string]any) json.RawMessage {
	if len(drift) == 0 {
		return nil
	}
	b, _ := json.Marshal(drift)
	return json.RawMessage(b)
}

func (c Checker) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}
