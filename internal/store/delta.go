package store

import (
	"fmt"
	"math"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

type UnitStatusChange struct {
	Unit       string `json:"unit"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

type FieldChange struct {
	Field string `json:"field"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

type incidentDelta struct {
	UnitsAdded    []model.Unit       `json:"units_added,omitempty"`
	UnitsRemoved  []model.Unit       `json:"units_removed,omitempty"`
	UnitsChanged  []UnitStatusChange `json:"units_changed,omitempty"`
	FieldsChanged []FieldChange      `json:"fields_changed,omitempty"`
}

func (d incidentDelta) Empty() bool {
	return len(d.UnitsAdded) == 0 &&
		len(d.UnitsRemoved) == 0 &&
		len(d.UnitsChanged) == 0 &&
		len(d.FieldsChanged) == 0
}

func diffUnits(prior, next []model.Unit) incidentDelta {
	delta := incidentDelta{}
	priorByUnit := unitsByName(prior)
	nextByUnit := unitsByName(next)

	for unit, nextUnit := range nextByUnit {
		priorUnit, ok := priorByUnit[unit]
		if !ok {
			delta.UnitsAdded = append(delta.UnitsAdded, nextUnit)
			continue
		}
		if priorUnit.Status != nextUnit.Status {
			delta.UnitsChanged = append(delta.UnitsChanged, UnitStatusChange{
				Unit:       unit,
				FromStatus: priorUnit.Status,
				ToStatus:   nextUnit.Status,
			})
		}
	}

	for unit, priorUnit := range priorByUnit {
		if _, ok := nextByUnit[unit]; !ok {
			delta.UnitsRemoved = append(delta.UnitsRemoved, priorUnit)
		}
	}

	return delta
}

func unitsByName(units []model.Unit) map[string]model.Unit {
	out := make(map[string]model.Unit, len(units))
	for _, unit := range units {
		if unit.Unit == "" {
			continue
		}
		out[unit.Unit] = unit
	}
	return out
}

func appendFieldChange(delta *incidentDelta, field, prior, next string) {
	if prior != next {
		delta.FieldsChanged = append(delta.FieldsChanged, FieldChange{Field: field, From: prior, To: next})
	}
}

func appendFloatFieldChange(delta *incidentDelta, field string, prior, next float64) {
	if math.Abs(prior-next) > 0.000001 {
		appendFieldChange(delta, field, fmt.Sprintf("%.6f", prior), fmt.Sprintf("%.6f", next))
	}
}
