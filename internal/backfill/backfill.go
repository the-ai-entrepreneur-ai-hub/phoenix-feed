package backfill

import (
	"encoding/json"
	"strings"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
)

type rawPhoenixFeature struct {
	Attributes struct {
		Units *string `json:"Units"`
	} `json:"attributes"`
}

func UnitsFromRaw(raw []byte) ([]model.Unit, bool, error) {
	var feature rawPhoenixFeature
	if err := json.Unmarshal(raw, &feature); err != nil {
		return nil, false, err
	}
	if feature.Attributes.Units == nil {
		return []model.Unit{}, false, nil
	}
	return phxfire.ParseUnits(*feature.Attributes.Units), true, nil
}

func HasSmashedStatus(units []model.Unit) bool {
	for _, unit := range units {
		if strings.Contains(unit.Status, ":") {
			return true
		}
	}
	return false
}
