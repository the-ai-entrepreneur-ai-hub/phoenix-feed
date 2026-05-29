package dispatchparser

import (
	"reflect"
	"testing"
)

func TestRealWorldLiveTranscripts(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantGate    bool
		wantNature  string
		wantAddress string
		wantUnits   []ExpectedUnit
	}{
		{
			name:        "medic seizure clark",
			text:        "Medic 258, CDEC 4, seizure. 1223 East Clark Street.",
			wantGate:    true,
			wantNature:  "Seizure",
			wantAddress: "1223 East Clark Street",
			wantUnits:   []ExpectedUnit{{UnitName: "Medic 258", UnitType: "Medic"}},
		},
		{
			name:        "split house number fire alarm",
			text:        "Engine 442 CDEC 12 residential fire alarm. 182 31 East Coronado Cave Court.",
			wantGate:    true,
			wantNature:  "Residential Fire Alarm",
			wantAddress: "18231 East Coronado Cave Court",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 442", UnitType: "Engine"}},
		},
		{
			name:        "punctuated baseline address",
			text:        "Engine 204, CDEC 3, ill person, BLS, 146, West Baseline Road.",
			wantGate:    true,
			wantNature:  "Ill Person",
			wantAddress: "146 West Baseline Road",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 204", UnitType: "Engine"}},
		},
		{
			name:        "medic fall hagley",
			text:        "Medic 220 CDEC 5 fall 537 South Hagley Road.",
			wantGate:    true,
			wantNature:  "Fall",
			wantAddress: "537 South Hagley Road",
			wantUnits:   []ExpectedUnit{{UnitName: "Medic 220", UnitType: "Medic"}},
		},
		{
			name:        "ignore leading lieutenant unit",
			text:        "LT 201. Medic 204, CDEC 3, difficulty breathing. 330 East Broadway Road.",
			wantGate:    true,
			wantNature:  "Difficulty Breathing",
			wantAddress: "330 East Broadway Road",
			wantUnits:   []ExpectedUnit{{UnitName: "Medic 204", UnitType: "Medic"}},
		},
		{
			name:        "signal butte over under",
			text:        "Engine 414 CDEC 5 motor vehicle accident with motorcycle. Signal Butte and 824 over under.",
			wantGate:    true,
			wantNature:  "Motor Vehicle Accident With Motorcycle",
			wantAddress: "Signal Butte and 824",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 414", UnitType: "Engine"}},
		},
		{
			name:        "sea deck injured person",
			text:        "Copy. Medic 261, Sea Deck 4, injured person, 1617 North Ironwood Drive.",
			wantGate:    true,
			wantNature:  "Injured Person",
			wantAddress: "1617 North Ironwood Drive",
			wantUnits:   []ExpectedUnit{{UnitName: "Medic 261", UnitType: "Medic"}},
		},
		{
			name:     "truncated cardiac address is not promoted",
			text:     "Engine 203 and Medic 2203. CDEC 3 cardiac problems. 520 ...",
			wantGate: false,
			// The transcript is truncated before a street or intersection, so
			// promoting it would create an ungeocodable location.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, reason := ParseTranscript(tt.text, 0.91)
			if ok != tt.wantGate {
				t.Fatalf("gate = %v reason=%q, want %v", ok, reason, tt.wantGate)
			}
			if !tt.wantGate {
				return
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

func TestAuditFalseNegativeRecallExamples(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		wantNature  string
		wantAddress string
		wantUnits   []ExpectedUnit
	}{
		{
			name:        "k deck ill person",
			text:        "Engine 207 K-Deck 7 Hill Person 2525 East Southern Avenue Unit 205",
			wantNature:  "Ill Person",
			wantAddress: "2525 East Southern Avenue",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 207", UnitType: "Engine"}},
		},
		{
			name:        "hyphenated amr and punctuated house number",
			text:        "A-M-R-2-0-7. Sea Deck 12. Fall. C-L-F. 154-35. East Cabern Drive",
			wantNature:  "Fall",
			wantAddress: "154-35 East Cabern Drive",
			wantUnits:   []ExpectedUnit{{UnitName: "AMR 207", UnitType: "AMR"}},
		},
		{
			name:        "nature address unit channel order",
			text:        "Level of consciousness. 4862 East Main Street. Unit B91. Medic 2220. Seabex 5",
			wantNature:  "Level of Consciousness",
			wantAddress: "4862 East Main Street",
			wantUnits:   []ExpectedUnit{{UnitName: "Medic 2220", UnitType: "Medic"}},
		},
		{
			name:        "unit after address",
			text:        "Sea deck 4. Animal issue. 4323 North Winchester Road. Engine 261",
			wantNature:  "Animal Issue",
			wantAddress: "4323 North Winchester Road",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 261", UnitType: "Engine"}},
		},
		{
			name:        "undirected street intersection",
			text:        "Engine 962 CDEC 7 working fire Hardy Drive and Minton Drive",
			wantNature:  "Working Fire",
			wantAddress: "Hardy Drive and Minton Drive",
			wantUnits:   []ExpectedUnit{{UnitName: "Engine 962", UnitType: "Engine"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, reason := ParseTranscript(tt.text, 0.91)
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

func TestAuditFalseNegativeExamplesStillRequireCDEC(t *testing.T) {
	tests := []string{
		"Fire Channel B3. 211 natural gas leak. 555 West Iron Avenue. Unit 101",
		"Fire channel A7. 962 with fire. Hardy Drive and Minton Drive",
		"Engine 57. Fire channel A9. House fire. 8504 South 22nd",
	}

	for _, text := range tests {
		t.Run(text, func(t *testing.T) {
			if got, ok, reason := ParseTranscript(text, 0.91); ok {
				t.Fatalf("gate passed with parsed=%+v, want rejection for non-CDEC transcript; reason=%q", got, reason)
			}
		})
	}
}
