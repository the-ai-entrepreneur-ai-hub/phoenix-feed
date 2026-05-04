package store

import (
	"testing"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

func TestDiffUnitsDetectsAddedRemovedAndChanged(t *testing.T) {
	prior := []model.Unit{
		{Unit: "E2203", Status: "Dispatched"},
		{Unit: "L24", Status: "On Scene"},
	}
	next := []model.Unit{
		{Unit: "E2203", Status: "On Scene"},
		{Unit: "BC2", Status: "Assigned"},
	}

	delta := diffUnits(prior, next)

	if len(delta.UnitsAdded) != 1 || delta.UnitsAdded[0].Unit != "BC2" {
		t.Fatalf("added = %+v", delta.UnitsAdded)
	}
	if len(delta.UnitsRemoved) != 1 || delta.UnitsRemoved[0].Unit != "L24" {
		t.Fatalf("removed = %+v", delta.UnitsRemoved)
	}
	if len(delta.UnitsChanged) != 1 {
		t.Fatalf("changed = %+v", delta.UnitsChanged)
	}
	change := delta.UnitsChanged[0]
	if change.Unit != "E2203" || change.FromStatus != "Dispatched" || change.ToStatus != "On Scene" {
		t.Fatalf("change = %+v", change)
	}
	if delta.Empty() {
		t.Fatal("delta should not be empty")
	}
}

func TestDiffUnitsUnchangedRepeatIsEmpty(t *testing.T) {
	prior := []model.Unit{{Unit: "E2203", Status: "On Scene"}}
	next := []model.Unit{{Unit: "E2203", Status: "On Scene"}}

	delta := diffUnits(prior, next)

	if !delta.Empty() {
		t.Fatalf("delta = %+v, want empty", delta)
	}
}

func TestDiffUnitsIgnoresBlankUnitKeys(t *testing.T) {
	prior := []model.Unit{{Unit: "", Status: "Dispatched"}}
	next := []model.Unit{{Unit: "", Status: "On Scene"}}

	delta := diffUnits(prior, next)

	if !delta.Empty() {
		t.Fatalf("delta = %+v, want empty", delta)
	}
}
