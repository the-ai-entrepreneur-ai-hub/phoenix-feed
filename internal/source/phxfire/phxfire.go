// Package phxfire implements source.Source against the Phoenix Fire
// Department's ArcGIS REST endpoint:
//
//	https://maps.phoenix.gov/phxfire/rest/services/Active_Incidents__Public/MapServer/0/query
//
// The endpoint is unauthenticated, has no published rate limit, and returns
// ESRI feature JSON. We pass outSR={"wkid":4326} so the server reprojects to
// WGS84 before responding (saves us from having to reimplement Arizona State
// Plane Central Zone projection in Go).
package phxfire

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

const (
	SourceName    = "phoenix-fire-mapserver"
	ParserVersion = "phx-fire-2026-05"

	// outSR is sent as a JSON-encoded object ({"wkid":4326}) rather than the
	// integer shorthand. Phoenix's ArcGIS server intermittently 500s on the
	// integer form (observed 2026-05-05) but accepts the JSON form reliably.
	defaultQueryURL = "https://maps.phoenix.gov/phxfire/rest/services/Active_Incidents__Public/MapServer/0/query" +
		"?where=1%3D1&outFields=*&f=json&outSR=%7B%22wkid%22%3A4326%7D"
	defaultTimeout = 15 * time.Second

	// Phoenix's IIS/ArcGIS Web Adaptor returns HTTP 500 for a User-Agent
	// containing a non-ASCII section-sign byte sequence (observed 2026-05-05
	// from app-ingester-1). Keep this header ASCII-only.
	defaultUserAgent = "phoenix-feed/0.1 (+architecture.md section 9)"
)

var expectedFields = []string{
	"OBJECTID",
	"Incident",
	"Nature",
	"NatureDesc",
	"Units",
	"Channel",
	"SymbolCode",
	"Date",
	"GenLocInfo",
}

var natureDescOverrides = map[string]string{
	"962":   "Vehicle Crash",
	"962A":  "Vehicle Crash",
	"962BC": "Crash Involving Bicycle",
	"962P":  "Crash Involving Pedestrian",
	"962X":  "Crash Requiring Extrication",
	"962MC": "Crash Involving Motorcycle",
}

var unitTypePrefixes = []struct {
	prefix   string
	unitType string
}{
	{prefix: "NEDC", unitType: "Division Chief"},
	{prefix: "NDC", unitType: "Division Chief"},
	{prefix: "SDC", unitType: "Division Chief"},
	{prefix: "WDC", unitType: "Division Chief"},
	{prefix: "EDC", unitType: "Division Chief"},
	{prefix: "BC", unitType: "Battalion Chief"},
	{prefix: "BR", unitType: "Brush truck"},
	{prefix: "HM", unitType: "Hazmat unit"},
	{prefix: "HR", unitType: "Heavy Rescue"},
	{prefix: "DR", unitType: "Drone"},
	{prefix: "PI", unitType: "Public Information"},
	{prefix: "LT", unitType: "Light truck"},
	{prefix: "E", unitType: "Engine"},
	{prefix: "L", unitType: "Ladder / Truck"},
	{prefix: "M", unitType: "Medic / Ambulance"},
	{prefix: "S", unitType: "Squad / Paramedic"},
	{prefix: "R", unitType: "Rescue"},
}

// Client is the Source.
type Client struct {
	queryURL   string
	httpClient *http.Client
}

// New returns a Client configured with sane defaults.
func New() *Client {
	return &Client{
		queryURL:   defaultQueryURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// DefaultQueryURL returns the production ArcGIS query URL.
func DefaultQueryURL() string {
	return defaultQueryURL
}

// ExpectedFields returns the Phoenix Fire fields the parser requires.
func ExpectedFields() []string {
	out := make([]string, len(expectedFields))
	copy(out, expectedFields)
	return out
}

// WithURL overrides the query URL — useful for tests pointing at fixtures.
func (c *Client) WithURL(u string) *Client {
	c.queryURL = u
	return c
}

// Name implements source.Source.
func (c *Client) Name() string { return SourceName }

// ParserVersion implements source.Source.
func (c *Client) ParserVersion() string { return ParserVersion }

// Poll fetches and parses the active incidents feed.
func (c *Client) Poll(ctx context.Context) model.PollResult {
	res := model.PollResult{
		Source:        SourceName,
		RequestURL:    c.queryURL,
		StartedAt:     time.Now().UTC(),
		ParserVersion: ParserVersion,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.queryURL, nil)
	if err != nil {
		res.FinishedAt = time.Now().UTC()
		res.Err = fmt.Errorf("build request: %w", err)
		return res
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := c.httpClient.Do(req)
	res.FinishedAt = time.Now().UTC()
	res.LatencyMS = int(res.FinishedAt.Sub(res.StartedAt).Milliseconds())
	if err != nil {
		res.Err = fmt.Errorf("http: %w", err)
		return res
	}
	defer resp.Body.Close()
	res.StatusCode = resp.StatusCode

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Err = fmt.Errorf("read body: %w", err)
		return res
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Err = fmt.Errorf("unexpected status %d", resp.StatusCode)
		return res
	}

	sum := sha256.Sum256(body)
	res.PayloadSHA256 = hex.EncodeToString(sum[:])

	incidents, err := ParseFeatures(body)
	if err != nil {
		res.Err = fmt.Errorf("parse: %w", err)
		return res
	}
	res.Incidents = incidents
	return res
}

// rawResponse is the ESRI feature JSON shape we care about.
type rawResponse struct {
	Features []struct {
		Attributes struct {
			OBJECTID   int64  `json:"OBJECTID"`
			Incident   string `json:"Incident"`
			Nature     string `json:"Nature"`
			NatureDesc string `json:"NatureDesc"`
			Units      string `json:"Units"`
			Channel    string `json:"Channel"`
			SymbolCode string `json:"SymbolCode"`
			Date       int64  `json:"Date"` // epoch ms
			GenLocInfo string `json:"GenLocInfo"`
		} `json:"attributes"`
		Geometry struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"geometry"`
	} `json:"features"`
}

// parseFeatures decodes ESRI JSON and normalizes each feature into model.Incident.
// Returns an error on JSON parse failure; per-feature errors are skipped with
// the rest of the batch returned.
func parseFeatures(body []byte) ([]model.Incident, error) {
	return ParseFeatures(body)
}

// ParseFeatures decodes ESRI JSON and normalizes each feature into model.Incident.
// It is exported so the contract canary can verify the live payload against the
// production parser.
func ParseFeatures(body []byte) ([]model.Incident, error) {
	var raw rawResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	out := make([]model.Incident, 0, len(raw.Features))
	for _, f := range raw.Features {
		if f.Attributes.Incident == "" {
			// Without an incident ID we have no primary key. Drop and continue.
			continue
		}
		if !plausibleWGS84(f.Geometry.X, f.Geometry.Y) {
			// Defensive guard: outSR=4326 may have been silently dropped.
			// Don't insert nonsense coordinates.
			continue
		}

		featureBytes, _ := json.Marshal(f) // best effort; safe to ignore err

		natureCode := strings.TrimSpace(f.Attributes.Nature)
		natureDesc := cleanNatureDesc(natureCode, f.Attributes.NatureDesc)

		out = append(out, model.Incident{
			Source:       SourceName,
			IncidentID:   strings.TrimSpace(f.Attributes.Incident),
			NatureCode:   natureCode,
			NatureDesc:   natureDesc,
			Units:        parseUnits(f.Attributes.Units),
			Channel:      strings.TrimSpace(f.Attributes.Channel),
			SymbolCode:   strings.TrimSpace(f.Attributes.SymbolCode),
			LocationText: strings.TrimSpace(f.Attributes.GenLocInfo),
			Lon:          f.Geometry.X,
			Lat:          f.Geometry.Y,
			IncidentDate: time.UnixMilli(f.Attributes.Date).UTC(),
			Raw:          featureBytes,
		})
	}
	return out, nil
}

// ParseUnits decodes Phoenix's HTML-entity-encoded Units string.
func ParseUnits(raw string) []model.Unit {
	return parseUnits(raw)
}

// parseUnits decodes the HTML-entity-encoded space-delimited Units string.
// Phoenix emits "<unit>: <status>" pairs separated by regular spaces after
// HTML entity decoding. Status values can contain spaces, so token boundaries
// are detected by the next token ending in ":".
func parseUnits(raw string) []model.Unit {
	decoded := html.UnescapeString(raw)
	decoded = strings.ReplaceAll(decoded, ",", " ")
	tokens := strings.Fields(decoded)
	if len(tokens) == 0 {
		return []model.Unit{}
	}

	out := []model.Unit{}
	var unit string
	var statusParts []string
	flush := func() {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			return
		}
		status := collapseWhitespace(strings.Join(statusParts, " "))
		out = append(out, model.Unit{Unit: unit, Status: status, UnitType: UnitTypeForName(unit)})
	}

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if strings.HasSuffix(token, ":") {
			flush()
			unit = strings.TrimSuffix(token, ":")
			statusParts = statusParts[:0]
			continue
		}
		if unit == "" {
			unit = token
			continue
		}
		statusParts = append(statusParts, token)
	}
	flush()
	return out
}

func cleanNatureDesc(code, desc string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	desc = strings.TrimSpace(desc)
	override, ok := natureDescOverrides[code]
	if !ok {
		return desc
	}
	upperDesc := strings.ToUpper(desc)
	family := numericPrefix(code)
	if desc == "" || upperDesc == code || strings.HasPrefix(upperDesc, code) || (family != "" && strings.HasPrefix(upperDesc, family+" ")) {
		return override
	}
	return desc
}

func numericPrefix(s string) string {
	for i, r := range s {
		if r < '0' || r > '9' {
			return s[:i]
		}
	}
	return s
}

func UnitTypeForName(unit string) string {
	unit = strings.ToUpper(strings.TrimSpace(unit))
	for _, item := range unitTypePrefixes {
		if strings.HasPrefix(unit, item.prefix) {
			return item.unitType
		}
	}
	return "other"
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == ' ' {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return b.String()
}

func plausibleWGS84(x, y float64) bool {
	return x >= -180 && x <= 180 && y >= -90 && y <= 90 && (x != 0 || y != 0)
}

// Compile-time interface guard.
var _ interface {
	Name() string
	ParserVersion() string
	Poll(context.Context) model.PollResult
} = (*Client)(nil)

// errEmpty is reserved for future use when we need to distinguish "endpoint
// returned zero features" from an error. Currently the lifecycle layer makes
// that decision, not the parser.
var errEmpty = errors.New("no features")

var _ = errEmpty // silence linter while reserved
