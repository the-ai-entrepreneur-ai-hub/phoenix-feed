package api

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func incidentDetailHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := chi.URLParam(r, "source")
		incidentID := chi.URLParam(r, "incident_id")
		if source == "" || incidentID == "" {
			writeError(w, http.StatusBadRequest, "source and incident_id are required")
			return
		}

		result, err := st.GetIncidentDetail(r.Context(), source, incidentID)
		if err != nil {
			log.Error("get incident detail", "source", source, "incident_id", incidentID, "err", err)
			writeError(w, http.StatusInternalServerError, "query incident detail")
			return
		}
		if result.Incident == nil {
			writeError(w, http.StatusNotFound, "incident not found")
			return
		}
		if result.Meta.ParserVersion == "" {
			result.Meta.ParserVersion = cfg.DefaultParserVersion
		}
		enrichIncidentDetail(result.Incident)
		now := apiNow(cfg)
		applyIncidentFreshnessMeta(&result.Meta, now, result.Incident.IncidentDate)
		applySourceStatus(&result.Meta, now, cfg)
		result.Incident.LastConfirmedAt = result.Incident.LastSeenAt
		result.Incident.IsLastKnown = incidentIsLastKnown(result.Meta, result.Incident.LastSeenAt)

		writeJSON(w, http.StatusOK, result)
	}
}
