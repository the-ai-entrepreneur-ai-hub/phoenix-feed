package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const defaultStaleAfter = 10 * time.Minute

type healthResponse struct {
	State       string                          `json:"state"`
	DBReachable bool                            `json:"db_reachable"`
	ServerTime  time.Time                       `json:"server_time"`
	Sources     map[string]sourceHealthResponse `json:"sources"`
}

type sourceHealthResponse struct {
	LastSuccessAt       *time.Time            `json:"last_success_at"`
	SecondsSinceSuccess *int                  `json:"seconds_since_success"`
	ParserVersion       string                `json:"parser_version"`
	Canary              *canaryHealthResponse `json:"canary,omitempty"`
}

type canaryHealthResponse struct {
	CheckedAt *time.Time `json:"checked_at"`
	Passed    *bool      `json:"passed"`
	Drift     any        `json:"drift,omitempty"`
}

func healthHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := apiNow(cfg)
		staleAfter := cfg.StaleAfter
		if staleAfter == 0 {
			staleAfter = defaultStaleAfter
		}

		dbOK := st.Ping(r.Context()) == nil
		sourceRows, err := st.SourceHealth(r.Context(), cfg.Sources)
		if err != nil {
			log.Error("source health", "err", err)
			dbOK = false
		}

		body := healthResponse{
			State:       "ok",
			DBReachable: dbOK,
			ServerTime:  now,
			Sources:     map[string]sourceHealthResponse{},
		}

		if !dbOK {
			body.State = "down"
		}

		for _, source := range sourceRows {
			row := buildSourceHealthResponse(source, now)
			body.Sources[source.Source] = row

			if source.LastSuccessAt == nil || now.Sub(*source.LastSuccessAt) > staleAfter {
				body.State = "down"
				continue
			}
			if body.State != "down" && source.Canary != nil && source.Canary.Passed != nil && !*source.Canary.Passed {
				body.State = "degraded"
			}
		}

		status := http.StatusOK
		if body.State == "down" {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, body)
	}
}

func buildSourceHealthResponse(source store.SourceHealth, now time.Time) sourceHealthResponse {
	var seconds *int
	if source.LastSuccessAt != nil {
		v := int(now.Sub(*source.LastSuccessAt).Seconds())
		if v < 0 {
			v = 0
		}
		seconds = &v
	}

	row := sourceHealthResponse{
		LastSuccessAt:       source.LastSuccessAt,
		SecondsSinceSuccess: seconds,
		ParserVersion:       source.ParserVersion,
	}
	if source.Canary != nil {
		row.Canary = &canaryHealthResponse{
			CheckedAt: source.Canary.CheckedAt,
			Passed:    source.Canary.Passed,
			Drift:     source.Canary.Drift,
		}
	}
	return row
}

func apiNow(cfg Config) time.Time {
	if cfg.Now != nil {
		return cfg.Now().UTC()
	}
	return time.Now().UTC()
}
