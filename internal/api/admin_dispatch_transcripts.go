package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const (
	dispatchTranscriptMaxBodyBytes = 256 * 1024
	dispatchTranscriptMaxTextBytes = 16 * 1024
	dispatchTranscriptRecentLimit  = 50
	dispatchTranscriptStaleSeconds = 600
)

var (
	dispatchTranscriptMinCapturedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	dispatchWavFilenamePattern      = regexp.MustCompile(`^[A-Za-z0-9_]+\.wav$`)
)

func adminDispatchTranscriptHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validAdminBearer(r.Header.Get("Authorization"), cfg.AdminToken) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}
		if !isJSONContentType(r.Header.Get("Content-Type")) {
			writeError(w, http.StatusBadRequest, "content-type must be application/json")
			return
		}

		body, ok := readLimitedJSONBody(w, r)
		if !ok {
			return
		}
		input, err := dispatchTranscriptInputFromBody(body, apiNow(cfg))
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		id, duplicate, err := st.InsertDispatchTranscript(r.Context(), input)
		if err != nil {
			log.Error("insert dispatch transcript", "err", err)
			writeError(w, http.StatusInternalServerError, "insert dispatch transcript")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "duplicate": duplicate})
	}
}

func adminDispatchRecentTranscriptsHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validAdminBearer(r.Header.Get("Authorization"), cfg.AdminToken) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}

		rows, err := st.ListRecentDispatchTranscripts(r.Context(), dispatchTranscriptRecentLimit)
		if err != nil {
			log.Error("list dispatch transcripts", "err", err)
			writeError(w, http.StatusInternalServerError, "query dispatch transcripts")
			return
		}
		if rows == nil {
			rows = []store.DispatchTranscript{}
		}
		writeJSON(w, http.StatusOK, rows)
	}
}

func adminDispatchHealthHandler(st Store, cfg Config, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validAdminBearer(r.Header.Get("Authorization"), cfg.AdminToken) {
			writeError(w, http.StatusUnauthorized, "invalid admin token")
			return
		}

		now := apiNow(cfg)
		health, err := st.DispatchTranscriptHealth(r.Context(), now)
		if err != nil {
			log.Error("dispatch transcript health", "err", err)
			writeError(w, http.StatusInternalServerError, "query dispatch transcript health")
			return
		}
		writeJSON(w, http.StatusOK, buildDispatchHealthResponse(health, now))
	}
}

func adminDispatchRateLimitMiddleware(limiter *ratelimit.Limiter, configuredToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := parseAdminBearerToken(r.Header.Get("Authorization"))
			if !ok || !validAdminToken(token, configuredToken) {
				writeError(w, http.StatusUnauthorized, "invalid admin token")
				return
			}
			if limiter == nil {
				limiter = ratelimit.NewDefault()
			}

			identity := auth.Identity{
				Tier:      auth.TierPaid,
				KeyHash:   auth.HashKey(token),
				ClientID:  requestClientID(r),
				Anonymous: false,
			}
			decision := limiter.Allow(identity, ratelimit.ScopeAdminDispatch)
			if !decision.Allowed {
				retrySeconds := int(math.Ceil(decision.RetryAfter.Seconds()))
				if retrySeconds < 1 {
					retrySeconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func adminDispatchAccessLogMiddleware(log *slog.Logger, route string) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			requestID := dispatchRequestID(r)
			w.Header().Set("X-Request-ID", requestID)
			recorder := &dispatchStatusRecorder{ResponseWriter: w, status: http.StatusOK}
			defer func() {
				log.Info("admin dispatch request",
					"request_id", requestID,
					"method", r.Method,
					"path", route,
					"status", recorder.status,
					"latency_ms", time.Since(start).Milliseconds(),
				)
			}()
			next.ServeHTTP(recorder, r)
		})
	}
}

type dispatchStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *dispatchStatusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func dispatchRequestID(r *http.Request) string {
	if requestID := strings.TrimSpace(r.Header.Get("X-Request-ID")); requestID != "" {
		return requestID
	}
	return fmt.Sprintf("dispatch-%d", time.Now().UnixNano())
}

func readLimitedJSONBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, dispatchTranscriptMaxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return nil, false
		}
		writeError(w, http.StatusBadRequest, "read request body")
		return nil, false
	}
	return body, true
}

func dispatchTranscriptInputFromBody(body []byte, now time.Time) (store.DispatchTranscriptInsert, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return store.DispatchTranscriptInsert{}, errors.New("malformed json")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return store.DispatchTranscriptInsert{}, errors.New("malformed json")
	}

	wavFilename := strings.TrimSpace(stringField(payload, "wav_filename"))
	if wavFilename == "" {
		return store.DispatchTranscriptInsert{}, errors.New("wav_filename is required")
	}
	if err := validateDispatchWavFilename(wavFilename); err != nil {
		return store.DispatchTranscriptInsert{}, err
	}
	capturedAtRaw := strings.TrimSpace(stringField(payload, "captured_at"))
	if capturedAtRaw == "" {
		return store.DispatchTranscriptInsert{}, errors.New("captured_at is required")
	}
	capturedAt, err := time.Parse(time.RFC3339, capturedAtRaw)
	if err != nil {
		return store.DispatchTranscriptInsert{}, errors.New("captured_at must be RFC3339")
	}
	capturedAt = capturedAt.UTC()
	if err := validateDispatchCapturedAt(capturedAt, now); err != nil {
		return store.DispatchTranscriptInsert{}, err
	}

	primary := objectField(payload, "primary")
	secondary := objectField(payload, "secondary")
	verification := objectField(payload, "verification")
	domainKeywords := objectField(verification, "domain_keywords")
	displayText := stringField(payload, "display_text")
	if len(displayText) > dispatchTranscriptMaxTextBytes {
		return store.DispatchTranscriptInsert{}, fmt.Errorf("display_text must be %d bytes or less", dispatchTranscriptMaxTextBytes)
	}

	return store.DispatchTranscriptInsert{
		WavFilename:            wavFilename,
		CapturedAt:             capturedAt,
		AudioDurationSeconds:   numberField(payload, "audio_duration_s"),
		DisplayText:            displayText,
		PrimaryText:            stringField(primary, "text"),
		SecondaryText:          stringField(secondary, "text"),
		PrimaryModel:           stringField(primary, "model"),
		SecondaryModel:         stringField(secondary, "model"),
		PrimaryAvgLogprob:      numberField(primary, "avg_logprob"),
		VerificationConfidence: numberField(verification, "confidence"),
		VerificationAgreement:  numberField(verification, "agreement"),
		ReviewRecommended:      boolField(verification, "review_recommended"),
		DomainKeywordMatches:   stringSliceField(domainKeywords, "matches"),
		DomainKeywordRatio:     numberField(domainKeywords, "ratio"),
		RawPayload:             append(json.RawMessage(nil), body...),
	}, nil
}

func validateDispatchWavFilename(filename string) error {
	if strings.Contains(filename, "..") {
		return errors.New("wav_filename must not contain ..")
	}
	if strings.ContainsAny(filename, `/\`) {
		return errors.New("wav_filename must not contain path separators")
	}
	if !dispatchWavFilenamePattern.MatchString(filename) {
		return errors.New("wav_filename must match ^[A-Za-z0-9_]+\\.wav$")
	}
	return nil
}

func validateDispatchCapturedAt(capturedAt, now time.Time) error {
	capturedAt = capturedAt.UTC()
	now = now.UTC()
	if capturedAt.Before(dispatchTranscriptMinCapturedAt) {
		return errors.New("captured_at must be on or after 2024-01-01T00:00:00Z")
	}
	if capturedAt.After(now.Add(30 * time.Minute)) {
		return errors.New("captured_at must not be more than 30 minutes in the future")
	}
	return nil
}

type dispatchHealthResponse struct {
	LastReceivedAt                  *time.Time `json:"last_received_at"`
	LastReceivedAgeSeconds          *int       `json:"last_received_age_seconds"`
	RowsLastHour                    int        `json:"rows_last_hour"`
	RowsLast24h                     int        `json:"rows_last_24h"`
	HighConfidenceLastHour          int        `json:"high_confidence_last_hour"`
	LowConfidenceLastHour           int        `json:"low_confidence_last_hour"`
	ReviewRecommendedLastHour       int        `json:"review_recommended_last_hour"`
	ParserLastBatchAt               *time.Time `json:"parser_last_batch_at"`
	ParserRowsPromotedLastHour      int        `json:"parser_rows_promoted_last_hour"`
	ParserRowsGateFailedLastHour    int        `json:"parser_rows_gate_failed_last_hour"`
	ParserRowsGeocodeFailedLastHour int        `json:"parser_rows_geocode_failed_last_hour"`
	ParserBacklogUnparsed           int        `json:"parser_backlog_unparsed"`
	Status                          string     `json:"status"`
}

func buildDispatchHealthResponse(health store.DispatchTranscriptHealth, now time.Time) dispatchHealthResponse {
	status := "stale"
	var age *int
	if health.LastReceivedAt != nil {
		seconds := int(now.UTC().Sub(health.LastReceivedAt.UTC()).Seconds())
		if seconds < 0 {
			seconds = 0
		}
		age = &seconds
		if seconds <= dispatchTranscriptStaleSeconds {
			status = "ok"
		}
	}
	return dispatchHealthResponse{
		LastReceivedAt:                  health.LastReceivedAt,
		LastReceivedAgeSeconds:          age,
		RowsLastHour:                    health.RowsLastHour,
		RowsLast24h:                     health.RowsLast24h,
		HighConfidenceLastHour:          health.HighConfidenceLastHour,
		LowConfidenceLastHour:           health.LowConfidenceLastHour,
		ReviewRecommendedLastHour:       health.ReviewRecommendedLastHour,
		ParserLastBatchAt:               health.ParserLastBatchAt,
		ParserRowsPromotedLastHour:      health.ParserRowsPromotedLastHour,
		ParserRowsGateFailedLastHour:    health.ParserRowsGateFailedLastHour,
		ParserRowsGeocodeFailedLastHour: health.ParserRowsGeocodeFailedLastHour,
		ParserBacklogUnparsed:           health.ParserBacklogUnparsed,
		Status:                          status,
	}
}

func isJSONContentType(contentType string) bool {
	if strings.TrimSpace(contentType) == "" {
		return true
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	return mediaType == "application/json"
}

func objectField(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	value, ok := payload[key].(map[string]any)
	if !ok {
		return nil
	}
	return value
}

func stringField(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key].(string)
	if !ok {
		return ""
	}
	return value
}

func boolField(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	value, ok := payload[key].(bool)
	return ok && value
}

func numberField(payload map[string]any, key string) *float64 {
	if payload == nil {
		return nil
	}
	switch value := payload[key].(type) {
	case json.Number:
		floatValue, err := value.Float64()
		if err != nil {
			return nil
		}
		return &floatValue
	case float64:
		return &value
	default:
		return nil
	}
}

func stringSliceField(payload map[string]any, key string) []string {
	if payload == nil {
		return nil
	}
	values, ok := payload[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s, ok := value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
