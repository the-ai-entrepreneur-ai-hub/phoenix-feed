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
