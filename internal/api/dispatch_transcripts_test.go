package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/store"
)

func (f *fakeStore) InsertDispatchTranscript(_ context.Context, input store.DispatchTranscriptInsert) (int64, bool, error) {
	f.dispatchInsert = input
	return f.dispatchInsertID, f.dispatchInsertDuplicate, f.dispatchInsertErr
}

func (f *fakeStore) ListRecentDispatchTranscripts(_ context.Context, limit int) ([]store.DispatchTranscript, error) {
	f.dispatchRecentLimit = limit
	return f.dispatchRecent, f.dispatchRecentErr
}

func TestAdminDispatchTranscriptPostInsertsTranscript(t *testing.T) {
	capturedAt := "2026-05-28T07:10:43Z"
	st := &fakeStore{dispatchInsertID: 42}
	body := sampleDispatchTranscriptJSON("20260528_001043_unknown.wav", capturedAt)
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		ID        int64 `json:"id"`
		Duplicate bool  `json:"duplicate"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != 42 || response.Duplicate {
		t.Fatalf("response = %+v, want id=42 duplicate=false", response)
	}
	if st.dispatchInsert.WavFilename != "20260528_001043_unknown.wav" {
		t.Fatalf("wav_filename = %q", st.dispatchInsert.WavFilename)
	}
	if got := st.dispatchInsert.CapturedAt.Format(time.RFC3339); got != capturedAt {
		t.Fatalf("captured_at = %s, want %s", got, capturedAt)
	}
	if st.dispatchInsert.DisplayText != "3-5 Paul, debate with any movers or any units." {
		t.Fatalf("display_text = %q", st.dispatchInsert.DisplayText)
	}
	if st.dispatchInsert.PrimaryText != "3-5 Paul, debate with any movers or any units." {
		t.Fatalf("primary_text = %q", st.dispatchInsert.PrimaryText)
	}
	if st.dispatchInsert.PrimaryModel != "medium.en" {
		t.Fatalf("primary_model = %q", st.dispatchInsert.PrimaryModel)
	}
	if st.dispatchInsert.VerificationConfidence == nil || *st.dispatchInsert.VerificationConfidence != 0.368 {
		t.Fatalf("verification_confidence = %v", st.dispatchInsert.VerificationConfidence)
	}
	if len(st.dispatchInsert.DomainKeywordMatches) != 1 || st.dispatchInsert.DomainKeywordMatches[0] != "unit" {
		t.Fatalf("domain_keyword_matches = %#v", st.dispatchInsert.DomainKeywordMatches)
	}
	if !json.Valid(st.dispatchInsert.RawPayload) || !bytes.Contains(st.dispatchInsert.RawPayload, []byte(`"wav_filename"`)) {
		t.Fatalf("raw_payload was not preserved as JSON object: %s", string(st.dispatchInsert.RawPayload))
	}
}

func TestAdminDispatchTranscriptPostReturnsDuplicateExistingID(t *testing.T) {
	st := &fakeStore{dispatchInsertID: 42, dispatchInsertDuplicate: true}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(sampleDispatchTranscriptJSON("20260528_001043_unknown.wav", "2026-05-28T07:10:43Z")))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var response struct {
		ID        int64 `json:"id"`
		Duplicate bool  `json:"duplicate"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.ID != 42 || !response.Duplicate {
		t.Fatalf("response = %+v, want id=42 duplicate=true", response)
	}
}

func TestAdminDispatchTranscriptPostRequiresBearerToken(t *testing.T) {
	for _, tc := range []struct {
		name          string
		adminToken    string
		authorization string
	}{
		{name: "missing header", adminToken: "admin-secret"},
		{name: "wrong bearer", adminToken: "admin-secret", authorization: "Bearer wrong"},
		{name: "unset configured token", adminToken: "", authorization: "Bearer admin-secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(sampleDispatchTranscriptJSON("20260528_001043_unknown.wav", "2026-05-28T07:10:43Z")))
			req.Header.Set("Content-Type", "application/json")
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			rr := httptest.NewRecorder()

			Router(&fakeStore{}, Config{AdminToken: tc.adminToken}, slog.Default()).ServeHTTP(rr, req)

			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminDispatchTranscriptPostRejectsMalformedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(`{"wav_filename":`))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminDispatchTranscriptPostRejectsMissingRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "missing wav filename", body: `{"captured_at":"2026-05-28T07:10:43Z","display_text":"test"}`},
		{name: "empty wav filename", body: `{"wav_filename":" ","captured_at":"2026-05-28T07:10:43Z","display_text":"test"}`},
		{name: "missing captured at", body: `{"wav_filename":"20260528_001043_unknown.wav","display_text":"test"}`},
		{name: "bad captured at", body: `{"wav_filename":"20260528_001043_unknown.wav","captured_at":"not-a-time","display_text":"test"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer admin-secret")
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
			}
		})
	}
}

func TestAdminDispatchTranscriptPostRejectsOversizedBody(t *testing.T) {
	body := fmt.Sprintf(`{"wav_filename":"20260528_001043_unknown.wav","captured_at":"2026-05-28T07:10:43Z","display_text":"%s"}`, strings.Repeat("x", 257*1024))
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminDispatchTranscriptPostRateLimitsAfterSixtyRequestsPerToken(t *testing.T) {
	router := Router(&fakeStore{dispatchInsertID: 42}, Config{AdminToken: "admin-secret"}, slog.Default())

	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(sampleDispatchTranscriptJSON(fmt.Sprintf("20260528_0010%02d_unknown.wav", i), "2026-05-28T07:10:43Z")))
		req.Header.Set("Authorization", "Bearer admin-secret")
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200: %s", i+1, rr.Code, rr.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(sampleDispatchTranscriptJSON("20260528_001999_unknown.wav", "2026-05-28T07:10:43Z")))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("request 61 status = %d, want 429: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminDispatchTranscriptPostReturnsServerErrorOnStoreFailure(t *testing.T) {
	st := &fakeStore{dispatchInsertErr: errors.New("db unavailable")}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/dispatch/transcript", strings.NewReader(sampleDispatchTranscriptJSON("20260528_001043_unknown.wav", "2026-05-28T07:10:43Z")))
	req.Header.Set("Authorization", "Bearer admin-secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500: %s", rr.Code, rr.Body.String())
	}
}

func TestAdminDispatchRecentTranscriptsReturnsLast50(t *testing.T) {
	rows := make([]store.DispatchTranscript, 50)
	newest := time.Date(2026, 5, 28, 8, 0, 0, 0, time.UTC)
	for i := range rows {
		capturedAt := newest.Add(-time.Duration(i) * time.Minute)
		receivedAt := capturedAt.Add(5 * time.Second)
		rows[i] = store.DispatchTranscript{
			ID:                     int64(i + 1),
			WavFilename:            fmt.Sprintf("20260528_%06d_unknown.wav", i),
			CapturedAt:             capturedAt,
			ReceivedAt:             receivedAt,
			DisplayText:            fmt.Sprintf("dispatch %d", i),
			VerificationConfidence: ptrFloat64(float64(i) / 100),
			ReviewRecommended:      i%2 == 0,
		}
	}
	st := &fakeStore{dispatchRecent: rows}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/dispatch/transcripts/recent", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	rr := httptest.NewRecorder()

	Router(st, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if st.dispatchRecentLimit != 50 {
		t.Fatalf("recent limit = %d, want 50", st.dispatchRecentLimit)
	}
	var body []map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 50 {
		t.Fatalf("body length = %d, want 50", len(body))
	}
	if body[0]["display_text"] != "dispatch 0" || body[49]["display_text"] != "dispatch 49" {
		t.Fatalf("body order first=%v last=%v", body[0]["display_text"], body[49]["display_text"])
	}
}

func TestAdminDispatchRecentTranscriptsRequiresBearerToken(t *testing.T) {
	for _, authorization := range []string{"", "Bearer wrong"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/dispatch/transcripts/recent", nil)
		if authorization != "" {
			req.Header.Set("Authorization", authorization)
		}
		rr := httptest.NewRecorder()

		Router(&fakeStore{}, Config{AdminToken: "admin-secret"}, slog.Default()).ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("authorization %q status = %d, want 401: %s", authorization, rr.Code, rr.Body.String())
		}
	}
}

func sampleDispatchTranscriptJSON(wavFilename, capturedAt string) string {
	return fmt.Sprintf(`{
		"wav": %q,
		"wav_filename": %q,
		"captured_at": %q,
		"audio_duration_s": 5.3,
		"display_text": "3-5 Paul, debate with any movers or any units.",
		"display_text_source": "cleaned",
		"primary": {
			"model": "medium.en",
			"text": "3-5 Paul, debate with any movers or any units.",
			"avg_logprob": -0.998,
			"language": "en",
			"language_probability": 1
		},
		"secondary": {
			"model": "distil-large-v3",
			"text": ""
		},
		"verification": {
			"agreement": 1.0,
			"confidence": 0.368,
			"review_recommended": true,
			"domain_keywords": {
				"matches": ["unit"],
				"ratio": 0.1
			}
		},
		"preprocessing": {"sr": 16000}
	}`, wavFilename, wavFilename, capturedAt)
}

func ptrFloat64(v float64) *float64 {
	return &v
}
