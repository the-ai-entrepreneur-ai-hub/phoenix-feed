package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/auth"
	"github.com/abusedmindset/phoenix-feed/internal/ratelimit"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const (
	dispatchTranscriptMaxBodyBytes = 256 * 1024
	dispatchTranscriptRecentLimit  = 50
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
		input, err := dispatchTranscriptInputFromBody(body)
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

func adminDispatchRateLimitMiddleware(limiter *ratelimit.Limiter, configuredToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := parseAdminBearerToken(r.Header.Get("Authorization"))
			if !ok || !validAdminBearer(r.Header.Get("Authorization"), configuredToken) {
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
				w.Header().Set("Retry-After", "1")
				writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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

func dispatchTranscriptInputFromBody(body []byte) (store.DispatchTranscriptInsert, error) {
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
	capturedAtRaw := strings.TrimSpace(stringField(payload, "captured_at"))
	if capturedAtRaw == "" {
		return store.DispatchTranscriptInsert{}, errors.New("captured_at is required")
	}
	capturedAt, err := time.Parse(time.RFC3339, capturedAtRaw)
	if err != nil {
		return store.DispatchTranscriptInsert{}, errors.New("captured_at must be RFC3339")
	}

	primary := objectField(payload, "primary")
	secondary := objectField(payload, "secondary")
	verification := objectField(payload, "verification")
	domainKeywords := objectField(verification, "domain_keywords")

	return store.DispatchTranscriptInsert{
		WavFilename:            wavFilename,
		CapturedAt:             capturedAt.UTC(),
		AudioDurationSeconds:   numberField(payload, "audio_duration_s"),
		DisplayText:            stringField(payload, "display_text"),
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
