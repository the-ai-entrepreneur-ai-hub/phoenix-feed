package phxfire

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

const sampleResponse = `{
	"geometryType": "esriGeometryPoint",
	"spatialReference": {"wkid": 4326},
	"fields": [
		{"name":"OBJECTID"},{"name":"Incident"},{"name":"Nature"},
		{"name":"NatureDesc"},{"name":"Units"},{"name":"Channel"},
		{"name":"SymbolCode"},{"name":"Date"},{"name":"GenLocInfo"}
	],
	"features": [
		{
			"attributes": {
				"OBJECTID": 3159367,
				"Incident": "F26198635",
				"Nature": "WF",
				"NatureDesc": "REPORTD WORKING FIRE     ",
				"Units": "E2203:&#160;On&#160;Scene,L24:&#160;Dispatched",
				"Channel": "B3",
				"SymbolCode": "sc006-fire",
				"Date": 1777884118000,
				"GenLocInfo": "600 S COUNTRY CLUB DR ,MES"
			},
			"geometry": { "x": -111.84006, "y": 33.40284 }
		}
	]
}`

const productionSuccessResponse = `{"displayFieldName":"Incident","fieldAliases":{"OBJECTID":"OBJECTID","Incident":"Incident","Nature":"Nature","NatureDesc":"NatureDesc","Units":"Units","Channel":"Channel","SymbolCode":"SymbolCode","Date":"Date","GenLocInfo":"GenLocInfo"},"geometryType":"esriGeometryPoint","spatialReference":{"wkid":4326,"latestWkid":4326},"fields":[{"name":"OBJECTID","type":"esriFieldTypeOID","alias":"OBJECTID"},{"name":"Incident","type":"esriFieldTypeString","alias":"Incident","length":9},{"name":"Nature","type":"esriFieldTypeString","alias":"Nature","length":50},{"name":"NatureDesc","type":"esriFieldTypeString","alias":"NatureDesc","length":255},{"name":"Units","type":"esriFieldTypeString","alias":"Units","length":2000},{"name":"Channel","type":"esriFieldTypeString","alias":"Channel","length":50},{"name":"SymbolCode","type":"esriFieldTypeString","alias":"SymbolCode","length":255},{"name":"Date","type":"esriFieldTypeDate","alias":"Date","length":8},{"name":"GenLocInfo","type":"esriFieldTypeString","alias":"GenLocInfo","length":2000}],"features":[]}`

func TestParseUnits_StateMachine(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []model.Unit
	}{
		{
			name: "single unit",
			raw:  "E2203:&#160;On&#160;Scene",
			want: []model.Unit{{Unit: "E2203", Status: "On Scene", UnitType: "Engine"}},
		},
		{
			name: "legacy comma still tolerated",
			raw:  "E2203:&#160;On&#160;Scene,L24:&#160;Dispatched",
			want: []model.Unit{
				{Unit: "E2203", Status: "On Scene", UnitType: "Engine"},
				{Unit: "L24", Status: "Dispatched", UnitType: "Ladder / Truck"},
			},
		},
		{
			name: "production two responding units",
			raw:  "BR701:&#160;Responding E701:&#160;Responding",
			want: []model.Unit{
				{Unit: "BR701", Status: "Responding", UnitType: "Brush truck"},
				{Unit: "E701", Status: "Responding", UnitType: "Engine"},
			},
		},
		{
			name: "production on scene hazmat sample",
			raw:  "E41:&#160;On&#160;Scene HM41:&#160;On&#160;Scene",
			want: []model.Unit{
				{Unit: "E41", Status: "On Scene", UnitType: "Engine"},
				{Unit: "HM41", Status: "On Scene", UnitType: "Hazmat unit"},
			},
		},
		{
			name: "multi word command status",
			raw:  "BC2:&#160;Responding BC601:&#160;Command DR1:&#160;Responding E12:&#160;On&#160;Scene",
			want: []model.Unit{
				{Unit: "BC2", Status: "Responding", UnitType: "Battalion Chief"},
				{Unit: "BC601", Status: "Command", UnitType: "Battalion Chief"},
				{Unit: "DR1", Status: "Responding", UnitType: "Drone"},
				{Unit: "E12", Status: "On Scene", UnitType: "Engine"},
			},
		},
		{
			name: "large smashed dispatch sample",
			raw:  "BC3:&#160;Dispatched BC601:&#160;Dispatched C957N:&#160;Dispatched DR1:&#160;Dispatched E13:&#160;Dispatched E28:&#160;Dispatched HR144:&#160;Dispatched HT44:&#160;Dispatched NDC:&#160;Dispatched PI3:&#160;Dispatched S28:&#160;Dispatched",
			want: []model.Unit{
				{Unit: "BC3", Status: "Dispatched", UnitType: "Battalion Chief"},
				{Unit: "BC601", Status: "Dispatched", UnitType: "Battalion Chief"},
				{Unit: "C957N", Status: "Dispatched", UnitType: "other"},
				{Unit: "DR1", Status: "Dispatched", UnitType: "Drone"},
				{Unit: "E13", Status: "Dispatched", UnitType: "Engine"},
				{Unit: "E28", Status: "Dispatched", UnitType: "Engine"},
				{Unit: "HR144", Status: "Dispatched", UnitType: "Heavy Rescue"},
				{Unit: "HT44", Status: "Dispatched", UnitType: "other"},
				{Unit: "NDC", Status: "Dispatched", UnitType: "Division Chief"},
				{Unit: "PI3", Status: "Dispatched", UnitType: "Public Information"},
				{Unit: "S28", Status: "Dispatched", UnitType: "Squad / Paramedic"},
			},
		},
		{
			name: "weird spacing and non-breaking hyphen",
			raw:  "\t E25:&#160;Leaving&#160;For&#160;Hospital   M‑174:&#160;On&#160;Scene\n",
			want: []model.Unit{
				{Unit: "E25", Status: "Leaving For Hospital", UnitType: "Engine"},
				{Unit: "M‑174", Status: "On Scene", UnitType: "Medic / Ambulance"},
			},
		},
		{
			name: "empty input",
			raw:  "",
			want: []model.Unit{},
		},
		{
			name: "whitespace only",
			raw:  " \t\n ",
			want: []model.Unit{},
		},
		{
			name: "no colon best effort",
			raw:  "E25",
			want: []model.Unit{{Unit: "E25", Status: "", UnitType: "Engine"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseUnits(tt.raw)
			assertUnits(t, got, tt.want)
			if len(tt.want) == 0 && got == nil {
				t.Fatal("empty units must be a non-nil slice")
			}
		})
	}
}

func TestParseUnits_EmptyJSONSerializesAsArray(t *testing.T) {
	got := parseUnits("")
	body, err := json.Marshal(struct {
		Units []model.Unit `json:"units"`
	}{Units: got})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"units":[]}` {
		t.Fatalf("json = %s, want empty units array", string(body))
	}
}

func assertUnits(t *testing.T, got, want []model.Unit) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("units length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unit %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestParseFeatures_Sample(t *testing.T) {
	incs, err := parseFeatures([]byte(sampleResponse))
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("want 1 incident, got %d", len(incs))
	}
	got := incs[0]
	if got.IncidentID != "F26198635" {
		t.Errorf("IncidentID = %q, want F26198635", got.IncidentID)
	}
	if got.NatureCode != "WF" {
		t.Errorf("NatureCode = %q", got.NatureCode)
	}
	if got.NatureDesc != "REPORTD WORKING FIRE" {
		t.Errorf("NatureDesc not trimmed: %q", got.NatureDesc)
	}
	if got.Lon != -111.84006 || got.Lat != 33.40284 {
		t.Errorf("coords = %v,%v", got.Lon, got.Lat)
	}
	if got.IncidentDate.UnixMilli() != 1777884118000 {
		t.Errorf("date wrong: %v", got.IncidentDate)
	}
	if len(got.Units) != 2 {
		t.Errorf("units count = %d", len(got.Units))
	}
	if got.Source != SourceName {
		t.Errorf("source not set: %q", got.Source)
	}
}

func TestParseFeatures_NatureDescriptionOverrides(t *testing.T) {
	body := `{"features":[
		{"attributes":{"Incident":"F1","Nature":"962","NatureDesc":"962","Date":1777884118000},"geometry":{"x":-112.1,"y":33.5}},
		{"attributes":{"Incident":"F2","Nature":"962BC","NatureDesc":"962 INV BICYCLE","Date":1777884118000},"geometry":{"x":-112.2,"y":33.6}},
		{"attributes":{"Incident":"F3","Nature":"962P","NatureDesc":"Meaningful Phoenix Label","Date":1777884118000},"geometry":{"x":-112.3,"y":33.7}}
	]}`
	incs, err := ParseFeatures([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := incs[0].NatureDesc; got != "Vehicle Crash" {
		t.Fatalf("962 desc = %q, want Vehicle Crash", got)
	}
	if got := incs[1].NatureDesc; got != "Crash Involving Bicycle" {
		t.Fatalf("962BC desc = %q, want Crash Involving Bicycle", got)
	}
	if got := incs[2].NatureDesc; got != "Meaningful Phoenix Label" {
		t.Fatalf("meaningful desc was overridden: %q", got)
	}
}

func TestParseFeatures_ProductionSuccessPayload(t *testing.T) {
	incs, err := ParseFeatures([]byte(productionSuccessResponse))
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 0 {
		t.Fatalf("Phoenix production payload currently has zero features, got %d incidents", len(incs))
	}
}

func TestParseFeatures_RejectsImplausibleCoords(t *testing.T) {
	body := `{"features":[{"attributes":{"Incident":"X1","Date":1},"geometry":{"x":750000,"y":900000}}]}`
	incs, err := parseFeatures([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 0 {
		t.Errorf("expected implausible coord to be dropped, got %v", incs)
	}
}

func TestParseFeatures_DropsZeroOrEpochDate(t *testing.T) {
	// A missing/zero upstream Date must not become a 1970/2000 placeholder
	// (which the app renders as "01/01/00"). Such half-formed features are
	// dropped at parse time; they reappear once a real dispatch time exists.
	body := `{"features":[
		{"attributes":{"Incident":"OK","Nature":"WF","Date":1777884118000},"geometry":{"x":-112.1,"y":33.5}},
		{"attributes":{"Incident":"ZERO","Nature":"WF","Date":0},"geometry":{"x":-112.2,"y":33.6}},
		{"attributes":{"Incident":"EPOCH","Nature":"WF","Date":1},"geometry":{"x":-112.3,"y":33.7}},
		{"attributes":{"Incident":"NEG","Nature":"WF","Date":-5},"geometry":{"x":-112.4,"y":33.8}}
	]}`
	incs, err := ParseFeatures([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(incs) != 1 {
		t.Fatalf("want only the valid-date incident kept, got %d: %#v", len(incs), incs)
	}
	if incs[0].IncidentID != "OK" {
		t.Errorf("kept wrong incident: %q", incs[0].IncidentID)
	}
}

func TestPoll_UsesASCIIUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	c := New().WithURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := c.Poll(ctx)
	if !res.Success() {
		t.Fatalf("poll failed: %v (status=%d)", res.Err, res.StatusCode)
	}
	const wantUA = "phoenix-feed/0.1 (+architecture.md section 9)"
	if gotUA != wantUA {
		t.Fatalf("User-Agent = %q, want %q", gotUA, wantUA)
	}
	if !isASCII(gotUA) {
		t.Fatalf("User-Agent must be ASCII-only, got %q", gotUA)
	}
}

func TestPoll_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()

	c := New().WithURL(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res := c.Poll(ctx)
	if !res.Success() {
		t.Fatalf("poll failed: %v (status=%d)", res.Err, res.StatusCode)
	}
	if len(res.Incidents) != 1 {
		t.Errorf("want 1 incident, got %d", len(res.Incidents))
	}
	if res.PayloadSHA256 == "" {
		t.Errorf("payload hash empty")
	}
	if res.ParserVersion != ParserVersion {
		t.Errorf("parser version not set")
	}
}

func TestPollRejectsNonAuthoritativeResponses(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantClass  model.PollClassification
		wantReason string
		wantErr    bool
	}{
		{name: "non-2xx", status: 503, body: `{}`, wantClass: model.PollFailure, wantReason: "recent_failures", wantErr: true},
		{name: "malformed JSON", status: 200, body: `{`, wantClass: model.PollFailure, wantReason: "contract_invalid", wantErr: true},
		{name: "ArcGIS soft error", status: 200, body: `{"error":{"code":500,"message":"down"}}`, wantClass: model.PollFailure, wantReason: "contract_invalid", wantErr: true},
		{name: "missing features", status: 200, body: `{}`, wantClass: model.PollFailure, wantReason: "contract_invalid", wantErr: true},
		{name: "null features", status: 200, body: `{"features":null}`, wantClass: model.PollFailure, wantReason: "contract_invalid", wantErr: true},
		{name: "truncated", status: 200, body: fullContractPayload(`[]`, `,"exceededTransferLimit":true`), wantClass: model.PollDegradedSnapshot, wantReason: "snapshot_incomplete"},
		{name: "schema drift", status: 200, body: `{"geometryType":"esriGeometryPoint","spatialReference":{"wkid":4326},"fields":[],"features":[]}`, wantClass: model.PollDegradedSnapshot, wantReason: "contract_invalid"},
		{name: "spatial drift", status: 200, body: strings.Replace(sampleResponse, `"wkid": 4326`, `"wkid": 2868`, 1), wantClass: model.PollDegradedSnapshot, wantReason: "contract_invalid"},
		{name: "critical feature rejected", status: 200, body: fullContractPayload(`[{"attributes":{"Incident":"","Date":1777884118000},"geometry":{"x":-112.1,"y":33.5}}]`, ``), wantClass: model.PollDegradedSnapshot, wantReason: "snapshot_incomplete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			res := New().WithURL(srv.URL).Poll(context.Background())
			if res.Classification != tt.wantClass || res.Reason != tt.wantReason {
				t.Fatalf("classification/reason = %s/%s, want %s/%s (err=%v)", res.Classification, res.Reason, tt.wantClass, tt.wantReason, res.Err)
			}
			if (res.Err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr=%v", res.Err, tt.wantErr)
			}
			if res.Success() {
				t.Fatal("non-authoritative response reported success")
			}
		})
	}
}

func TestPollTimeoutIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(sampleResponse))
	}))
	defer srv.Close()
	c := New().WithURL(srv.URL)
	c.httpClient.Timeout = 5 * time.Millisecond
	res := c.Poll(context.Background())
	if res.Classification != model.PollFailure || !errors.Is(res.Err, context.DeadlineExceeded) {
		t.Fatalf("timeout result = %#v", res)
	}
}

func TestPollEmptyRequiresIndependentCountZero(t *testing.T) {
	for _, count := range []int{0, 1} {
		t.Run(fmt.Sprintf("count_%d", count), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("returnCountOnly") == "true" {
					_, _ = fmt.Fprintf(w, `{"count":%d}`, count)
					return
				}
				_, _ = w.Write([]byte(productionSuccessResponse))
			}))
			defer srv.Close()
			res := New().WithURL(srv.URL).Poll(context.Background())
			if count == 0 && !res.Success() {
				t.Fatalf("confirmed empty failed: class=%s reason=%s err=%v", res.Classification, res.Reason, res.Err)
			}
			if count != 0 && (res.Classification != model.PollDegradedSnapshot || res.Reason != "snapshot_incomplete") {
				t.Fatalf("mismatched empty = %s/%s", res.Classification, res.Reason)
			}
		})
	}
}

func TestPollEmptyRejectsMalformedCountConfirmation(t *testing.T) {
	for _, countBody := range []string{`{}`, `{"count":-1}`, `{"error":{"code":500}}`} {
		t.Run(countBody, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("returnCountOnly") == "true" {
					_, _ = w.Write([]byte(countBody))
					return
				}
				_, _ = w.Write([]byte(productionSuccessResponse))
			}))
			defer srv.Close()

			res := New().WithURL(srv.URL).Poll(context.Background())
			if res.Classification != model.PollDegradedSnapshot || res.Reason != "snapshot_incomplete" || res.Success() {
				t.Fatalf("malformed count accepted: %s/%s err=%v", res.Classification, res.Reason, res.Err)
			}
		})
	}
}

func TestPollFrozenNonEmptyAndChangedRecovery(t *testing.T) {
	now := time.Date(2026, 7, 21, 9, 0, 0, 0, time.UTC)
	body := sampleResponse
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := New().WithURL(srv.URL).withNow(func() time.Time { return now })

	for i := 1; i <= 3; i++ {
		res := c.Poll(context.Background())
		if i < 3 && !res.Success() {
			t.Fatalf("repeat %d unexpectedly non-authoritative: %s/%s", i, res.Classification, res.Reason)
		}
		if i == 3 && (res.Classification != model.PollDegradedSnapshot || res.Reason != "payload_frozen") {
			t.Fatalf("third repeat = %s/%s", res.Classification, res.Reason)
		}
		now = now.Add(time.Minute)
	}

	body = strings.Replace(sampleResponse, "F26198635", "F26198636", 1)
	res := c.Poll(context.Background())
	if !res.Success() {
		t.Fatalf("changed payload did not recover: %s/%s err=%v", res.Classification, res.Reason, res.Err)
	}
}

func TestPollSuddenCollapseRequiresMatchingCount(t *testing.T) {
	features := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		features = append(features, fmt.Sprintf(`{"attributes":{"Incident":"F%d","Nature":"WF","Date":1777884118000},"geometry":{"x":-112.%d,"y":33.5}}`, i, i+1))
	}
	body := fullContractPayload(`[`+strings.Join(features, ",")+`]`, ``)
	count := 5
	countCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			countCalls++
			_, _ = fmt.Fprintf(w, `{"count":%d}`, count)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	c := New().WithURL(srv.URL)
	if res := c.Poll(context.Background()); !res.Success() {
		t.Fatalf("baseline poll failed: %s/%s err=%v", res.Classification, res.Reason, res.Err)
	}

	body = fullContractPayload(`[`+features[0]+`]`, ``)
	count = 1
	if res := c.Poll(context.Background()); !res.Success() {
		t.Fatalf("confirmed collapse failed: %s/%s err=%v", res.Classification, res.Reason, res.Err)
	}
	if countCalls != 1 {
		t.Fatalf("count calls = %d, want 1", countCalls)
	}

	body = fullContractPayload(`[]`, ``)
	count = 2
	res := c.Poll(context.Background())
	if res.Classification != model.PollDegradedSnapshot || res.Reason != "snapshot_incomplete" {
		t.Fatalf("count mismatch = %s/%s", res.Classification, res.Reason)
	}
}

func TestPollConfirmedEmptyIsExemptFromFrozenDetection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("returnCountOnly") == "true" {
			_, _ = w.Write([]byte(`{"count":0}`))
			return
		}
		_, _ = w.Write([]byte(productionSuccessResponse))
	}))
	defer srv.Close()
	c := New().WithURL(srv.URL)
	for i := 0; i < 5; i++ {
		if res := c.Poll(context.Background()); !res.Success() {
			t.Fatalf("empty repeat %d = %s/%s err=%v", i+1, res.Classification, res.Reason, res.Err)
		}
	}
}

func fullContractPayload(features, extra string) string {
	return `{"geometryType":"esriGeometryPoint","spatialReference":{"wkid":4326},"fields":[` +
		`{"name":"OBJECTID"},{"name":"Incident"},{"name":"Nature"},{"name":"NatureDesc"},` +
		`{"name":"Units"},{"name":"Channel"},{"name":"SymbolCode"},{"name":"Date"},{"name":"GenLocInfo"}],` +
		`"features":` + features + extra + `}`
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
