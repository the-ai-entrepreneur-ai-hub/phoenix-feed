package phxfire

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestParseUnits_HTMLEntityDecode(t *testing.T) {
	got := parseUnits("E2203:&#160;On&#160;Scene,L24:&#160;Dispatched")
	if len(got) != 2 {
		t.Fatalf("want 2 units, got %d (%v)", len(got), got)
	}
	if got[0].Unit != "E2203" || got[0].Status != "On Scene" {
		t.Errorf("unit 0 = %+v, want {E2203 On Scene}", got[0])
	}
	if got[1].Unit != "L24" || got[1].Status != "Dispatched" {
		t.Errorf("unit 1 = %+v, want {L24 Dispatched}", got[1])
	}
}

func TestParseUnits_EmptyAndMalformed(t *testing.T) {
	if got := parseUnits(""); len(got) != 0 {
		t.Errorf("empty input: want nil, got %v", got)
	}
	if got := parseUnits("E25"); len(got) != 1 || got[0].Unit != "E25" || got[0].Status != "" {
		t.Errorf("no colon: got %v", got)
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
