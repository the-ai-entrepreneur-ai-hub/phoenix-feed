package api

import (
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func enrichActiveIncidents(incidents []store.ActiveIncident) {
	for i := range incidents {
		incidents[i].NatureDesc = phxfire.NatureDescriptionFor(incidents[i].NatureCode, incidents[i].NatureDesc)
		incidents[i].Severity = phxfire.SeverityForCode(incidents[i].NatureCode)
	}
}

func enrichIncidentDetail(incident *store.IncidentDetail) {
	if incident == nil {
		return
	}
	incident.NatureDesc = phxfire.NatureDescriptionFor(incident.NatureCode, incident.NatureDesc)
	incident.Severity = phxfire.SeverityForCode(incident.NatureCode)
}
