package dispatchparser

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type gateFixture struct {
	Name        string         `json:"name"`
	Text        string         `json:"text"`
	Confidence  float64        `json:"confidence"`
	WantGate    bool           `json:"want_gate"`
	WantNature  string         `json:"want_nature"`
	WantAddress string         `json:"want_address"`
	WantChannel string         `json:"want_channel"`
	WantUnits   []ExpectedUnit `json:"want_units"`
}

func TestGateFixtures(t *testing.T) {
	fixtures := loadGateFixtures(t)
	if len(fixtures) < 30 {
		t.Fatalf("fixtures length = %d, want at least 30", len(fixtures))
	}

	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			got, ok, reason := ParseTranscript(fixture.Text, fixture.Confidence)
			if ok != fixture.WantGate {
				t.Fatalf("gate = %v reason=%q, want %v", ok, reason, fixture.WantGate)
			}
			if !fixture.WantGate {
				return
			}
			if got.Nature != fixture.WantNature {
				t.Fatalf("nature = %q, want %q", got.Nature, fixture.WantNature)
			}
			if len(got.Nature) > 50 {
				t.Fatalf("nature = %q length %d, want <= 50", got.Nature, len(got.Nature))
			}
			if fixture.WantChannel != "" && got.Channel != fixture.WantChannel {
				t.Fatalf("channel = %q, want %q", got.Channel, fixture.WantChannel)
			}
			if got.LocationText != fixture.WantAddress {
				t.Fatalf("location = %q, want %q", got.LocationText, fixture.WantAddress)
			}
			if !reflect.DeepEqual(got.Units, fixture.WantUnits) {
				t.Fatalf("units = %#v, want %#v", got.Units, fixture.WantUnits)
			}
		})
	}
}

func TestParseTranscriptPicksFirstRepeatedDispatch(t *testing.T) {
	text := "Engine 2510 CDEC 4 Overdose 2350 West Obispo Avenue. Engine 2510 CDEC 4 Overdose 2350 West Obispo Avenue."

	got, ok, reason := ParseTranscript(text, 0.95)
	if !ok {
		t.Fatalf("gate failed: %s", reason)
	}
	if got.Nature != "Overdose" || got.LocationText != "2350 West Obispo Avenue" {
		t.Fatalf("parsed = %+v", got)
	}
}

func TestParseTranscriptAnchorsOnFirstCompleteDispatchSegment(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantNature  string
		wantAddress string
		wantUnits   []ExpectedUnit
	}{
		{
			name:        "sdr 2482 fall before seizure",
			text:        "Engine 414 CDEC 4 fall 1364 E-suite Citrus Drive. Engine 206 CDEC 4 seizure 1940 East University Drive.",
			wantNature:  "Fall",
			wantAddress: "1364 East Citrus Drive",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 414", UnitType: "Engine"}},
		},
		{
			name:        "sdr 5952 seizure before stroke",
			text:        "Ladder 259 CDEC 12 seizure 3473 East Crescent Way. Medic 2207 CDEC 12 stroke 2505 West Plata Avenue.",
			wantNature:  "Seizure",
			wantAddress: "3473 East Crescent Way",
			wantUnits:   []ExpectedUnit{{UnitName: "Ladder 259", UnitType: "Ladder"}},
		},
		{
			name:        "sdr 6735 crash before fall and breathing call",
			text:        "Engine 414 CDEC 5 motor vehicle accident with motorcycle Signal Butte and 824 over under. Medic 220 CDEC 5 fall 537 South Hagley Road. Ladder 263 Sea Deck 4 difficulty breathing 447 East Broadway Avenue.",
			wantNature:  "Motor Vehicle Accident With Motorcycle",
			wantAddress: "Signal Butte and 824",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 414", UnitType: "Engine"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, reason := ParseTranscript(tt.text, 0.95)
			if !ok {
				t.Fatalf("gate failed: %s", reason)
			}
			if got.Nature != tt.wantNature {
				t.Fatalf("nature = %q, want %q", got.Nature, tt.wantNature)
			}
			if got.LocationText != tt.wantAddress {
				t.Fatalf("address = %q, want %q", got.LocationText, tt.wantAddress)
			}
			if !reflect.DeepEqual(got.Units, tt.wantUnits) {
				t.Fatalf("units = %#v, want %#v", got.Units, tt.wantUnits)
			}
		})
	}
}

func loadGateFixtures(t *testing.T) []gateFixture {
	t.Helper()
	body, err := os.ReadFile("testdata/gate_fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []gateFixture
	if err := json.Unmarshal(body, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}
