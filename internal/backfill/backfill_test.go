package backfill

import (
	"testing"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

func TestUnitsFromRawPhoenixFeature(t *testing.T) {
	raw := []byte(`{"attributes":{"Units":"E41:&#160;On&#160;Scene HM41:&#160;On&#160;Scene"}}`)

	units, hasUnits, err := UnitsFromRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !hasUnits {
		t.Fatal("hasUnits = false, want true")
	}
	want := []model.Unit{
		{Unit: "E41", Status: "On Scene", UnitType: "Engine"},
		{Unit: "HM41", Status: "On Scene", UnitType: "Hazmat unit"},
	}
	if len(units) != len(want) {
		t.Fatalf("units length = %d, want %d: %#v", len(units), len(want), units)
	}
	for i := range want {
		if units[i] != want[i] {
			t.Fatalf("unit %d = %#v, want %#v", i, units[i], want[i])
		}
	}
}

func TestUnitsFromRawEmptyUnitsReturnsNonNilSlice(t *testing.T) {
	units, hasUnits, err := UnitsFromRaw([]byte(`{"attributes":{"Units":"   "}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !hasUnits {
		t.Fatal("hasUnits = false, want true for present empty Units field")
	}
	if units == nil {
		t.Fatal("units = nil, want non-nil empty slice")
	}
	if len(units) != 0 {
		t.Fatalf("units length = %d, want 0", len(units))
	}
}

func TestHasSmashedStatus(t *testing.T) {
	before := []model.Unit{{Unit: "BR701", Status: "Responding E701: Responding"}}
	if !HasSmashedStatus(before) {
		t.Fatal("expected smashed status to be detected")
	}
	after := []model.Unit{
		{Unit: "BR701", Status: "Responding"},
		{Unit: "E701", Status: "Responding"},
	}
	if HasSmashedStatus(after) {
		t.Fatal("clean parsed units were reported as smashed")
	}
}
