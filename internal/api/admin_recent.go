package api

import (
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const (
	adminRecentDefaultHours = 48
	adminRecentMaxHours     = 48
	adminRecentMaxItems     = 500
)

func adminRecentIncidentsHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validAdminBearer(r.Header.Get("Authorization"), cfg.AdminToken) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}

		hours, err := parseAdminRecentHours(r.URL.Query().Get("hours"))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := st.ListRecentIncidents(r.Context(), store.RecentIncidentFilter{
			Hours: hours,
			Limit: adminRecentMaxItems,
		})
		if err != nil {
			log.Error("list recent incidents", "err", err)
			writeError(w, http.StatusInternalServerError, "query recent incidents")
			return
		}

		enrichActiveIncidents(result.Incidents)
		w.Header().Set("X-Total-Count", strconv.Itoa(result.TotalCount))
		w.Header().Set("X-Returned-Count", strconv.Itoa(len(result.Incidents)))
		writeJSON(w, http.StatusOK, result.Incidents)
	}
}

func validAdminBearer(header, configuredToken string) bool {
	configuredToken = strings.TrimSpace(configuredToken)
	if configuredToken == "" {
		return false
	}
	token, ok := parseAdminBearerToken(header)
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(configuredToken)) == 1
}

func parseAdminBearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", false
	}
	return token, true
}

func parseAdminRecentHours(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return adminRecentDefaultHours, nil
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours < 1 || hours > adminRecentMaxHours {
		return 0, fmt.Errorf("hours must be an integer from 1 through %d", adminRecentMaxHours)
	}
	return hours, nil
}
