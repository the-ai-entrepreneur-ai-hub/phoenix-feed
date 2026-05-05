package phxfire

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

const sampleResponse = `{
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

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
