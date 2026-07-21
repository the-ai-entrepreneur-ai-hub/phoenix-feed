package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

type healthResponse struct {
	State       string                          `json:"state"`
	DBReachable bool                            `json:"db_reachable"`
	ServerTime  time.Time                       `json:"server_time"`
	Sources     map[string]sourceHealthResponse `json:"sources"`
}

type sourceHealthResponse struct {
	Status              string                `json:"source_status"`
	StatusReason        string                `json:"source_status_reason"`
	LastSuccessAt       *time.Time            `json:"last_success_at"`
	LastAttemptAt       *time.Time            `json:"last_attempt_at"`
	SecondsSinceSuccess *int                  `json:"seconds_since_success"`
	ConsecutiveFailures int                   `json:"consecutive_failures"`
	FeatureCount        *int                  `json:"feature_count"`
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
			row := buildSourceHealthResponse(source, now, cfg)
			body.Sources[source.Source] = row

			if row.Status == sourceStatusDown {
				body.State = "down"
				continue
			}
			if body.State != "down" && row.Status == sourceStatusDegraded {
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

func buildSourceHealthResponse(source store.SourceHealth, now time.Time, cfg Config) sourceHealthResponse {
	var seconds *int
	if source.LastSuccessAt != nil {
		v := elapsedSeconds(now, *source.LastSuccessAt)
		seconds = &v
	}
	degradedAfter, downAfter, downFailures, frozenDownAfter := sourceThresholds(cfg)
	status, reason := classifySourceStatus(
		now, source.LastSuccessAt, source.ConsecutiveFailures,
		source.LatestClassification, source.LatestReason, source.LatestUnchangedSince,
		degradedAfter, downAfter, downFailures, frozenDownAfter,
	)
	if status == sourceStatusOK && source.Canary != nil && source.Canary.Passed != nil && !*source.Canary.Passed {
		status = sourceStatusDegraded
		reason = "contract_invalid"
	}

	row := sourceHealthResponse{
		Status:              status,
		StatusReason:        reason,
		LastSuccessAt:       source.LastSuccessAt,
		LastAttemptAt:       source.LastAttemptAt,
		SecondsSinceSuccess: seconds,
		ConsecutiveFailures: source.ConsecutiveFailures,
		FeatureCount:        source.LatestFeatureCount,
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
