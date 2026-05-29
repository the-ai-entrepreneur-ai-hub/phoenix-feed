// Package store wraps Postgres access. No ORM. Hand-written SQL keeps the
// schema visible at the call site, which matters more than the boilerplate
// it costs.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/source/phxfire"
)

type Store struct {
	pool *pgxpool.Pool
}

type BBox struct {
	MinLon float64
	MinLat float64
	MaxLon float64
	MaxLat float64
}

type Radius struct {
	Lat    float64
	Lon    float64
	Meters float64
}

type ActiveIncidentFilter struct {
	BBox   *BBox
	Radius *Radius
	Since  *time.Time
	Until  *time.Time
}

type RecentIncidentFilter struct {
	Hours int
	Limit int
}

type StalenessMeta struct {
	SourceLastSuccessAt  *time.Time `json:"source_last_success_at"`
	DataAgeSeconds       *int       `json:"data_age_seconds"`
	NewestIncidentAt     *time.Time `json:"newest_incident_at"`
	DataStalenessSeconds *int       `json:"data_staleness_seconds"`
	ParserVersion        string     `json:"parser_version"`
	Disclaimer           string     `json:"disclaimer,omitempty"`
	Attribution          string     `json:"attribution,omitempty"`
	RefreshMinSeconds    int        `json:"refresh_min_seconds,omitempty"`
	Tier                 string     `json:"tier,omitempty"`
}

type ActiveIncident struct {
	Source               string       `json:"source"`
	IncidentID           string       `json:"incident_id"`
	NatureCode           string       `json:"nature_code,omitempty"`
	NatureDesc           string       `json:"nature_desc,omitempty"`
	Severity             string       `json:"severity,omitempty"`
	Units                []model.Unit `json:"units"`
	Channel              string       `json:"channel,omitempty"`
	SymbolCode           string       `json:"symbol_code,omitempty"`
	LocationText         string       `json:"location_text,omitempty"`
	Lon                  float64      `json:"lon"`
	Lat                  float64      `json:"lat"`
	IncidentDate         time.Time    `json:"incident_date"`
	ReceivedAt           time.Time    `json:"received_at"`
	LastSeenAt           time.Time    `json:"last_seen_at"`
	SourceLastSuccessAt  *time.Time   `json:"source_last_success_at,omitempty"`
	SecondsSinceLastSeen int          `json:"seconds_since_last_seen"`
}

type ActiveIncidentsResult struct {
	Meta      StalenessMeta    `json:"meta"`
	Incidents []ActiveIncident `json:"incidents"`
}

type RecentIncidentsResult struct {
	TotalCount int
	Incidents  []ActiveIncident
}

type DispatchTranscriptInsert struct {
	WavFilename            string
	CapturedAt             time.Time
	AudioDurationSeconds   *float64
	DisplayText            string
	PrimaryText            string
	SecondaryText          string
	PrimaryModel           string
	SecondaryModel         string
	PrimaryAvgLogprob      *float64
	VerificationConfidence *float64
	VerificationAgreement  *float64
	ReviewRecommended      bool
	DomainKeywordMatches   []string
	DomainKeywordRatio     *float64
	RawPayload             json.RawMessage
}

type DispatchTranscript struct {
	ID                     int64     `json:"id"`
	WavFilename            string    `json:"wav_filename"`
	CapturedAt             time.Time `json:"captured_at"`
	ReceivedAt             time.Time `json:"received_at"`
	DisplayText            string    `json:"display_text"`
	VerificationConfidence *float64  `json:"verification_confidence"`
	ReviewRecommended      bool      `json:"review_recommended"`
	ParsedIncidentID       *int64    `json:"parsed_incident_id"`
}

type DispatchTranscriptHealth struct {
	LastReceivedAt                  *time.Time
	RowsLastHour                    int
	RowsLast24h                     int
	HighConfidenceLastHour          int
	LowConfidenceLastHour           int
	ReviewRecommendedLastHour       int
	ParserLastBatchAt               *time.Time
	ParserRowsPromotedLastHour      int
	ParserRowsGateFailedLastHour    int
	ParserRowsGeocodeFailedLastHour int
	ParserBacklogUnparsed           int
}

type IncidentDetail struct {
	Source       string       `json:"source"`
	IncidentID   string       `json:"incident_id"`
	NatureCode   string       `json:"nature_code,omitempty"`
	NatureDesc   string       `json:"nature_desc,omitempty"`
	Severity     string       `json:"severity,omitempty"`
	Units        []model.Unit `json:"units"`
	Channel      string       `json:"channel,omitempty"`
	SymbolCode   string       `json:"symbol_code,omitempty"`
	LocationText string       `json:"location_text,omitempty"`
	Lon          float64      `json:"lon"`
	Lat          float64      `json:"lat"`
	IncidentDate time.Time    `json:"incident_date"`
	ReceivedAt   time.Time    `json:"received_at"`
	LastSeenAt   time.Time    `json:"last_seen_at"`
	ClearedAt    *time.Time   `json:"cleared_at,omitempty"`
	ReopenCount  int          `json:"reopen_count"`
}

type UnitObservation struct {
	Unit            string    `json:"unit"`
	Status          string    `json:"status"`
	FirstObservedAt time.Time `json:"first_observed_at"`
	LastObservedAt  time.Time `json:"last_observed_at"`
	FirstPollID     int64     `json:"first_poll_id"`
	LastPollID      int64     `json:"last_poll_id"`
}

type IncidentEvent struct {
	ID         int64           `json:"id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	PollID     *int64          `json:"poll_id,omitempty"`
	Delta      json.RawMessage `json:"delta,omitempty"`
}

type IncidentDetailResult struct {
	Meta        StalenessMeta     `json:"meta"`
	Incident    *IncidentDetail   `json:"incident"`
	UnitHistory []UnitObservation `json:"unit_history"`
	Events      []IncidentEvent   `json:"events"`
}

type PublicStats struct {
	CurrentActiveCount  int            `json:"current_active_count"`
	TodayTotalIncidents int            `json:"today_total_incidents"`
	TodayByCategory     map[string]int `json:"today_by_category"`
	Last24hTotal        int            `json:"last_24h_total"`
	ActiveUnitsNow      int            `json:"active_units_now"`
	DataAgeSeconds      int            `json:"data_age_seconds"`
	Tier                string         `json:"tier"`
	SourceUpdatedAt     time.Time      `json:"-"`
}

type CanaryHealth struct {
	CheckedAt     *time.Time
	Passed        *bool
	Drift         json.RawMessage
	ParserVersion string
}

type SourceHealth struct {
	Source        string
	LastSuccessAt *time.Time
	ParserVersion string
	Canary        *CanaryHealth
}

type ContractCanaryResult struct {
	Source         string          `json:"source"`
	CheckedAt      time.Time       `json:"checked_at"`
	ExpectedFields []string        `json:"expected_fields"`
	ActualFields   []string        `json:"actual_fields"`
	OutputSR       *int            `json:"output_sr,omitempty"`
	FeatureCount   int             `json:"feature_count"`
	GeomPlausible  bool            `json:"geom_plausible"`
	Drift          json.RawMessage `json:"drift,omitempty"`
	Passed         bool            `json:"passed"`
	ParserVersion  string          `json:"parser_version"`
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgx pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgx ping: %w", err)
	}
	return &Store{pool: pool}, nil
}

func NewWithPool(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Close() { s.pool.Close() }

// RecordPoll inserts a row into source_polls and returns the new poll_id.
// Called both on success and failure so the audit trail is complete.
func (s *Store) RecordPoll(ctx context.Context, r model.PollResult) (int64, error) {
	return s.recordPoll(ctx, r, r.Success(), pollErrorString(r.Err))
}

// RecordPollPending records a successful upstream poll as not yet applied.
// Lifecycle flips it to success only after backend writes complete.
func (s *Store) RecordPollPending(ctx context.Context, r model.PollResult) (int64, error) {
	return s.recordPoll(ctx, r, false, "poll apply pending")
}

func (s *Store) recordPoll(ctx context.Context, r model.PollResult, success bool, errStr string) (int64, error) {
	const q = `
		INSERT INTO source_polls (
			source, request_url, started_at, finished_at, status_code,
			latency_ms, feature_count, payload_sha256, parser_version,
			success, error
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,NULLIF($11,''))
		RETURNING poll_id`

	var id int64
	err := s.pool.QueryRow(ctx, q,
		r.Source, r.RequestURL, r.StartedAt, r.FinishedAt, r.StatusCode,
		r.LatencyMS, len(r.Incidents), r.PayloadSHA256, r.ParserVersion,
		success, errStr,
	).Scan(&id)
	return id, err
}

func (s *Store) MarkPollSucceeded(ctx context.Context, pollID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE source_polls
		SET success = TRUE, error = NULL
		WHERE poll_id = $1`, pollID,
	)
	return err
}

func (s *Store) MarkPollFailed(ctx context.Context, pollID int64, applyErr error) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE source_polls
		SET success = FALSE, error = NULLIF($2, '')
		WHERE poll_id = $1`, pollID, pollErrorString(applyErr),
	)
	return err
}

func pollErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ListActiveIncidents returns active incidents plus source staleness metadata.
func (s *Store) ListActiveIncidents(ctx context.Context, filter ActiveIncidentFilter) (ActiveIncidentsResult, error) {
	meta, err := s.activeStalenessMeta(ctx)
	if err != nil {
		return ActiveIncidentsResult{}, err
	}

	q := `
		SELECT source, incident_id, nature_code, nature_desc, units, channel,
		       symbol_code, location_text, ST_X(geom), ST_Y(geom), incident_date,
		       received_at, last_seen_at, source_last_success_at,
		       seconds_since_last_seen
		FROM active_incidents
		WHERE geom IS NOT NULL`
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if filter.BBox != nil {
		minLon := arg(filter.BBox.MinLon)
		minLat := arg(filter.BBox.MinLat)
		maxLon := arg(filter.BBox.MaxLon)
		maxLat := arg(filter.BBox.MaxLat)
		q += fmt.Sprintf(" AND geom && ST_MakeEnvelope(%s,%s,%s,%s,4326)", minLon, minLat, maxLon, maxLat)
	}
	if filter.Radius != nil {
		lon := arg(filter.Radius.Lon)
		lat := arg(filter.Radius.Lat)
		meters := arg(filter.Radius.Meters)
		q += fmt.Sprintf(" AND ST_DWithin(geom::geography, ST_SetSRID(ST_MakePoint(%s,%s),4326)::geography, %s)", lon, lat, meters)
	}
	if filter.Since != nil {
		q += " AND incident_date >= " + arg(*filter.Since)
	}
	if filter.Until != nil {
		q += " AND incident_date <= " + arg(*filter.Until)
	}
	q += " ORDER BY incident_date DESC, incident_id"

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return ActiveIncidentsResult{}, err
	}
	defer rows.Close()

	incidents := []ActiveIncident{}
	for rows.Next() {
		inc := ActiveIncident{Units: []model.Unit{}}
		var unitsBytes []byte
		if err := rows.Scan(
			&inc.Source, &inc.IncidentID, &inc.NatureCode, &inc.NatureDesc, &unitsBytes, &inc.Channel,
			&inc.SymbolCode, &inc.LocationText, &inc.Lon, &inc.Lat, &inc.IncidentDate,
			&inc.ReceivedAt, &inc.LastSeenAt, &inc.SourceLastSuccessAt, &inc.SecondsSinceLastSeen,
		); err != nil {
			return ActiveIncidentsResult{}, err
		}
		if len(unitsBytes) > 0 {
			if err := json.Unmarshal(unitsBytes, &inc.Units); err != nil {
				return ActiveIncidentsResult{}, fmt.Errorf("decode units for %s/%s: %w", inc.Source, inc.IncidentID, err)
			}
			if inc.Units == nil {
				inc.Units = []model.Unit{}
			}
		}
		incidents = append(incidents, inc)
	}
	if err := rows.Err(); err != nil {
		return ActiveIncidentsResult{}, err
	}

	return ActiveIncidentsResult{Meta: meta, Incidents: incidents}, nil
}

// ListRecentIncidents returns a newest-first admin view over the same incident
// store used by the active feed. Cleared incidents remain eligible while their
// incident_date falls inside the requested window.
func (s *Store) ListRecentIncidents(ctx context.Context, filter RecentIncidentFilter) (RecentIncidentsResult, error) {
	if filter.Hours <= 0 {
		return RecentIncidentsResult{}, fmt.Errorf("recent incident hours must be positive")
	}
	if filter.Limit <= 0 {
		filter.Limit = 500
	}

	rows, err := s.pool.Query(ctx, `
		SELECT source, incident_id, nature_code, nature_desc, units, channel,
		       symbol_code, location_text, ST_X(geom), ST_Y(geom), incident_date,
		       received_at, last_seen_at,
		       (SELECT started_at FROM source_polls WHERE success = TRUE ORDER BY started_at DESC LIMIT 1) AS source_last_success_at,
		       GREATEST(0, EXTRACT(EPOCH FROM (NOW() - last_seen_at))::int) AS seconds_since_last_seen,
		       COUNT(*) OVER()::int AS total_count
		FROM incidents
		WHERE geom IS NOT NULL
		  AND incident_date >= NOW() - ($1::int * INTERVAL '1 hour')
		ORDER BY incident_date DESC, incident_id
		LIMIT $2`, filter.Hours, filter.Limit,
	)
	if err != nil {
		return RecentIncidentsResult{}, err
	}
	defer rows.Close()

	result := RecentIncidentsResult{Incidents: []ActiveIncident{}}
	for rows.Next() {
		inc := ActiveIncident{Units: []model.Unit{}}
		var unitsBytes []byte
		var natureCode, natureDesc, channel, symbolCode, locationText sql.NullString
		var lastSuccess sql.NullTime
		var totalCount int
		if err := rows.Scan(
			&inc.Source, &inc.IncidentID, &natureCode, &natureDesc, &unitsBytes, &channel,
			&symbolCode, &locationText, &inc.Lon, &inc.Lat, &inc.IncidentDate,
			&inc.ReceivedAt, &inc.LastSeenAt, &lastSuccess, &inc.SecondsSinceLastSeen, &totalCount,
		); err != nil {
			return RecentIncidentsResult{}, err
		}
		inc.NatureCode = nullString(natureCode)
		inc.NatureDesc = nullString(natureDesc)
		inc.Channel = nullString(channel)
		inc.SymbolCode = nullString(symbolCode)
		inc.LocationText = nullString(locationText)
		if lastSuccess.Valid {
			inc.SourceLastSuccessAt = &lastSuccess.Time
		}
		if len(unitsBytes) > 0 {
			if err := json.Unmarshal(unitsBytes, &inc.Units); err != nil {
				return RecentIncidentsResult{}, fmt.Errorf("decode units for %s/%s: %w", inc.Source, inc.IncidentID, err)
			}
			if inc.Units == nil {
				inc.Units = []model.Unit{}
			}
		}
		result.TotalCount = totalCount
		result.Incidents = append(result.Incidents, inc)
	}
	if err := rows.Err(); err != nil {
		return RecentIncidentsResult{}, err
	}
	return result, nil
}

const insertDispatchTranscriptSQL = `
	INSERT INTO dispatch_transcripts (
		wav_filename, captured_at, audio_duration_s, display_text,
		primary_text, secondary_text, primary_model, secondary_model,
		primary_avg_logprob, verification_confidence, verification_agreement,
		review_recommended, domain_keyword_matches, domain_keyword_ratio,
		raw_payload
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15::jsonb
	)
	ON CONFLICT (wav_filename) DO UPDATE
	SET wav_filename = dispatch_transcripts.wav_filename
	RETURNING id, (xmax <> 0) AS duplicate`

const listRecentDispatchTranscriptsSQL = `
	SELECT id, wav_filename, captured_at, received_at, display_text,
	       verification_confidence::float8, review_recommended, parsed_incident_id
	FROM dispatch_transcripts
	ORDER BY received_at DESC
	LIMIT $1`

const dispatchTranscriptHealthSQL = `
	SELECT
		(SELECT received_at FROM dispatch_transcripts ORDER BY received_at DESC LIMIT 1),
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE received_at >= $1)::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE received_at >= $2)::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE received_at >= $1 AND verification_confidence >= $3)::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE received_at >= $1 AND (verification_confidence IS NULL OR verification_confidence < $3))::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE received_at >= $1 AND review_recommended = TRUE)::int,
		(SELECT parsed_at FROM dispatch_transcripts WHERE parsed_at IS NOT NULL ORDER BY parsed_at DESC LIMIT 1),
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE parsed_at >= $1 AND parsed_incident_id IS NOT NULL)::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE parsed_at >= $1 AND parsed_incident_id IS NULL AND NOT (` + dispatchTranscriptGateSQL + `))::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE parsed_at >= $1 AND parsed_incident_id IS NULL AND (` + dispatchTranscriptGateSQL + `))::int,
		(SELECT COUNT(*) FROM dispatch_transcripts WHERE parsed_at IS NULL)::int`

const dispatchTranscriptGateSQL = `
	verification_confidence >= 0.80
	AND display_text ~* '(^|[^[:alnum:]_])CDEC[[:space:]-]?[0-9]+([^[:alnum:]_]|$)'
	AND display_text ~* '(^|[^[:alnum:]_])(engine|ladder|battalion|rescue|squad|truck|medic|chief|amr)[[:space:]-]?[0-9]+([^[:alnum:]_]|$)'
	AND (
		display_text ~* '(^|[^[:alnum:]_])[0-9]+(-[0-9]+)?[[:space:]]+(north|south|east|west|n|s|e|w)[[:space:]]+[a-z0-9]+([[:space:]-]+[a-z0-9]+){0,5}[[:space:]]+(avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)([^[:alnum:]_]|$)'
		OR display_text ~* '(^|[^[:alnum:]_])(north|south|east|west|n|s|e|w)[[:space:]]+[a-z0-9]+([[:space:]-]+[a-z0-9]+){0,5}[[:space:]]+(avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)[[:space:]]+(and|at|/)[[:space:]]+(north|south|east|west|n|s|e|w)[[:space:]]+[a-z0-9]+([[:space:]-]+[a-z0-9]+){0,5}[[:space:]]+(avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)([^[:alnum:]_]|$)'
		OR display_text ~* '(^|[^[:alnum:]_])(north|south|east|west|n|s|e|w)[[:space:]]+[0-9]+[a-z]{0,2}[[:space:]]+(avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)([^[:alnum:]_]|$)'
	)`

const dispatchHighConfidenceThreshold = 0.75

func (s *Store) InsertDispatchTranscript(ctx context.Context, input DispatchTranscriptInsert) (int64, bool, error) {
	var id int64
	var duplicate bool
	err := s.pool.QueryRow(ctx, insertDispatchTranscriptSQL,
		input.WavFilename,
		input.CapturedAt,
		input.AudioDurationSeconds,
		input.DisplayText,
		nullableString(input.PrimaryText),
		nullableString(input.SecondaryText),
		nullableString(input.PrimaryModel),
		nullableString(input.SecondaryModel),
		input.PrimaryAvgLogprob,
		input.VerificationConfidence,
		input.VerificationAgreement,
		input.ReviewRecommended,
		input.DomainKeywordMatches,
		input.DomainKeywordRatio,
		string(input.RawPayload),
	).Scan(&id, &duplicate)
	if err != nil {
		return 0, false, err
	}
	return id, duplicate, nil
}

func (s *Store) ListRecentDispatchTranscripts(ctx context.Context, limit int) ([]DispatchTranscript, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.pool.Query(ctx, listRecentDispatchTranscriptsSQL, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []DispatchTranscript{}
	for rows.Next() {
		var row DispatchTranscript
		var confidence sql.NullFloat64
		var parsedID sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.WavFilename,
			&row.CapturedAt,
			&row.ReceivedAt,
			&row.DisplayText,
			&confidence,
			&row.ReviewRecommended,
			&parsedID,
		); err != nil {
			return nil, err
		}
		if confidence.Valid {
			row.VerificationConfidence = &confidence.Float64
		}
		if parsedID.Valid {
			row.ParsedIncidentID = &parsedID.Int64
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) DispatchTranscriptHealth(ctx context.Context, now time.Time) (DispatchTranscriptHealth, error) {
	var health DispatchTranscriptHealth
	var lastReceived sql.NullTime
	var parserLastBatch sql.NullTime
	err := s.pool.QueryRow(ctx, dispatchTranscriptHealthSQL,
		now.UTC().Add(-1*time.Hour),
		now.UTC().Add(-24*time.Hour),
		dispatchHighConfidenceThreshold,
	).Scan(
		&lastReceived,
		&health.RowsLastHour,
		&health.RowsLast24h,
		&health.HighConfidenceLastHour,
		&health.LowConfidenceLastHour,
		&health.ReviewRecommendedLastHour,
		&parserLastBatch,
		&health.ParserRowsPromotedLastHour,
		&health.ParserRowsGateFailedLastHour,
		&health.ParserRowsGeocodeFailedLastHour,
		&health.ParserBacklogUnparsed,
	)
	if err != nil {
		return DispatchTranscriptHealth{}, err
	}
	if lastReceived.Valid {
		t := lastReceived.Time.UTC()
		health.LastReceivedAt = &t
	}
	if parserLastBatch.Valid {
		t := parserLastBatch.Time.UTC()
		health.ParserLastBatchAt = &t
	}
	return health, nil
}

func (s *Store) activeStalenessMeta(ctx context.Context) (StalenessMeta, error) {
	const q = `
		SELECT started_at, parser_version FROM source_polls
		WHERE success = TRUE
		ORDER BY started_at DESC LIMIT 1`
	var started time.Time
	var parserVersion string
	err := s.pool.QueryRow(ctx, q).Scan(&started, &parserVersion)
	if err == pgx.ErrNoRows {
		return StalenessMeta{}, nil
	}
	if err != nil {
		return StalenessMeta{}, err
	}
	age := int(math.Max(0, time.Since(started).Seconds()))
	return StalenessMeta{
		SourceLastSuccessAt: &started,
		DataAgeSeconds:      &age,
		ParserVersion:       parserVersion,
	}, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// GetIncidentDetail returns one incident with observed unit and event history.
func (s *Store) GetIncidentDetail(ctx context.Context, source, incidentID string) (IncidentDetailResult, error) {
	meta, err := s.activeStalenessMeta(ctx)
	if err != nil {
		return IncidentDetailResult{}, err
	}

	incident, err := s.getIncident(ctx, source, incidentID)
	if err != nil {
		return IncidentDetailResult{}, err
	}
	if incident == nil {
		return IncidentDetailResult{Meta: meta}, nil
	}

	units, err := s.getUnitHistory(ctx, source, incidentID)
	if err != nil {
		return IncidentDetailResult{}, err
	}
	events, err := s.getIncidentEvents(ctx, source, incidentID)
	if err != nil {
		return IncidentDetailResult{}, err
	}

	return IncidentDetailResult{
		Meta:        meta,
		Incident:    incident,
		UnitHistory: units,
		Events:      events,
	}, nil
}

func (s *Store) getIncident(ctx context.Context, source, incidentID string) (*IncidentDetail, error) {
	const q = `
		SELECT source, incident_id, nature_code, nature_desc, units, channel,
		       symbol_code, location_text, ST_X(geom), ST_Y(geom), incident_date,
		       received_at, last_seen_at, cleared_at, reopen_count
		FROM incidents
		WHERE source = $1 AND incident_id = $2`

	inc := IncidentDetail{Units: []model.Unit{}}
	var natureCode, natureDesc, channel, symbolCode, locationText sql.NullString
	var lon, lat sql.NullFloat64
	var clearedAt sql.NullTime
	var unitsBytes []byte
	err := s.pool.QueryRow(ctx, q, source, incidentID).Scan(
		&inc.Source, &inc.IncidentID, &natureCode, &natureDesc, &unitsBytes, &channel,
		&symbolCode, &locationText, &lon, &lat, &inc.IncidentDate,
		&inc.ReceivedAt, &inc.LastSeenAt, &clearedAt, &inc.ReopenCount,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get incident %s/%s: %w", source, incidentID, err)
	}

	inc.NatureCode = nullString(natureCode)
	inc.NatureDesc = nullString(natureDesc)
	inc.Channel = nullString(channel)
	inc.SymbolCode = nullString(symbolCode)
	inc.LocationText = nullString(locationText)
	inc.Lon = nullFloat(lon)
	inc.Lat = nullFloat(lat)
	if clearedAt.Valid {
		inc.ClearedAt = &clearedAt.Time
	}
	if len(unitsBytes) > 0 {
		if err := json.Unmarshal(unitsBytes, &inc.Units); err != nil {
			return nil, fmt.Errorf("decode incident units %s/%s: %w", source, incidentID, err)
		}
		if inc.Units == nil {
			inc.Units = []model.Unit{}
		}
	}
	return &inc, nil
}

func (s *Store) getUnitHistory(ctx context.Context, source, incidentID string) ([]UnitObservation, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT unit, status, first_observed_at, last_observed_at, first_poll_id, last_poll_id
		FROM incident_units
		WHERE source = $1 AND incident_id = $2
		ORDER BY first_observed_at ASC, id ASC`, source, incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query unit history %s/%s: %w", source, incidentID, err)
	}
	defer rows.Close()

	out := []UnitObservation{}
	for rows.Next() {
		var row UnitObservation
		if err := rows.Scan(&row.Unit, &row.Status, &row.FirstObservedAt, &row.LastObservedAt, &row.FirstPollID, &row.LastPollID); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) getIncidentEvents(ctx context.Context, source, incidentID string) ([]IncidentEvent, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, event_type, occurred_at, poll_id, delta
		FROM incident_events
		WHERE source = $1 AND incident_id = $2
		ORDER BY occurred_at ASC, id ASC`, source, incidentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query incident events %s/%s: %w", source, incidentID, err)
	}
	defer rows.Close()

	out := []IncidentEvent{}
	for rows.Next() {
		var row IncidentEvent
		var pollID sql.NullInt64
		var delta []byte
		if err := rows.Scan(&row.ID, &row.EventType, &row.OccurredAt, &pollID, &delta); err != nil {
			return nil, err
		}
		if pollID.Valid {
			row.PollID = &pollID.Int64
		}
		if len(delta) > 0 {
			row.Delta = json.RawMessage(delta)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// UpsertIncident applies a single observed incident, recording the
// (source, incident_id) row and resetting any clearing state. Called by the
// lifecycle layer for each feature observed in a successful poll.
func (s *Store) UpsertIncident(ctx context.Context, inc model.Incident, pollID int64, observedAt time.Time) error {
	return s.upsertIncidentWithHistory(ctx, inc, pollID, observedAt)
}

// MarkMissing flips on missing_since for incidents in this source that were
// NOT observed in this successful poll AND aren't already cleared. Idempotent.
func (s *Store) MarkMissing(ctx context.Context, source string, observedAt time.Time, observedIDs []string) error {
	const q = `
		UPDATE incidents
		SET missing_since = COALESCE(missing_since, $2)
		WHERE source = $1
		  AND cleared_at IS NULL
		  AND incident_id <> ALL($3::text[])`
	_, err := s.pool.Exec(ctx, q, source, observedAt, observedIDs)
	return err
}

// SweepCleared sets cleared_at for incidents that have been missing through
// at least minMisses consecutive successful polls. Returns the cleared
// incidents so the caller can emit incident_events.
func (s *Store) SweepCleared(ctx context.Context, source string, minMisses int) ([]ClearedIncident, error) {
	const q = `
		WITH miss_counts AS (
			SELECT i.incident_id,
			       (SELECT COUNT(*) FROM source_polls p
			        WHERE p.source = i.source
			          AND p.success = TRUE
			          AND p.started_at >= i.missing_since) AS misses
			FROM incidents i
			WHERE i.source = $1 AND i.cleared_at IS NULL AND i.missing_since IS NOT NULL
		)
		UPDATE incidents i
		SET cleared_at = i.missing_since
		FROM miss_counts m
		WHERE i.source = $1
		  AND i.incident_id = m.incident_id
		  AND m.misses >= $2
		RETURNING i.incident_id, i.cleared_at`

	rows, err := s.pool.Query(ctx, q, source, minMisses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cleared []ClearedIncident
	for rows.Next() {
		var item ClearedIncident
		if err := rows.Scan(&item.IncidentID, &item.ClearedAt); err != nil {
			return nil, err
		}
		item.Source = source
		cleared = append(cleared, item)
	}
	return cleared, rows.Err()
}

// LatestSuccessAt is exposed in API responses so clients can detect staleness.
func (s *Store) LatestSuccessAt(ctx context.Context, source string) (time.Time, error) {
	const q = `
		SELECT started_at FROM source_polls
		WHERE source = $1 AND success = TRUE
		ORDER BY started_at DESC LIMIT 1`
	var t time.Time
	err := s.pool.QueryRow(ctx, q, source).Scan(&t)
	if err == pgx.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

func (s *Store) PublicStats(ctx context.Context) (PublicStats, error) {
	const dayStart = `date_trunc('day', NOW() AT TIME ZONE 'America/Phoenix') AT TIME ZONE 'America/Phoenix'`
	stats := PublicStats{TodayByCategory: map[string]int{}}
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM active_incidents WHERE geom IS NOT NULL)::int,
			(SELECT COUNT(*) FROM incidents WHERE incident_date >= `+dayStart+`)::int,
			(SELECT COUNT(*) FROM incidents WHERE incident_date >= NOW() - INTERVAL '24 hours')::int,
			(SELECT COALESCE(SUM(jsonb_array_length(COALESCE(units, '[]'::jsonb))), 0)::int FROM active_incidents WHERE geom IS NOT NULL),
			COALESCE(EXTRACT(EPOCH FROM (NOW() - (
				SELECT started_at FROM source_polls
				WHERE success = TRUE
				ORDER BY started_at DESC
				LIMIT 1
			)))::int, 0)`,
	).Scan(&stats.CurrentActiveCount, &stats.TodayTotalIncidents, &stats.Last24hTotal, &stats.ActiveUnitsNow, &stats.DataAgeSeconds)
	if err != nil {
		return PublicStats{}, fmt.Errorf("public stats totals: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(NULLIF(nature_code, ''), 'UNKNOWN') AS nature_code, COUNT(*)::int
		FROM incidents
		WHERE incident_date >= `+dayStart+`
		GROUP BY 1
		ORDER BY 1`)
	if err != nil {
		return PublicStats{}, fmt.Errorf("public stats category counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var code string
		var count int
		if err := rows.Scan(&code, &count); err != nil {
			return PublicStats{}, err
		}
		label := phxfire.LabelForCode(code)
		if label == "" {
			label = "Unknown"
		}
		stats.TodayByCategory[label] += count
	}
	if err := rows.Err(); err != nil {
		return PublicStats{}, err
	}

	return stats, nil
}

// SourceHealth returns freshness and latest canary status for configured sources.
func (s *Store) SourceHealth(ctx context.Context, sources []string) ([]SourceHealth, error) {
	out := make([]SourceHealth, 0, len(sources))
	for _, source := range sources {
		row := SourceHealth{Source: source}

		var lastSuccess sql.NullTime
		var parserVersion sql.NullString
		err := s.pool.QueryRow(ctx, `
			SELECT started_at, parser_version
			FROM source_polls
			WHERE source = $1 AND success = TRUE
			ORDER BY started_at DESC
			LIMIT 1`, source,
		).Scan(&lastSuccess, &parserVersion)
		if err != nil && err != pgx.ErrNoRows {
			return nil, fmt.Errorf("latest source success %s: %w", source, err)
		}
		if lastSuccess.Valid {
			row.LastSuccessAt = &lastSuccess.Time
		}
		if parserVersion.Valid {
			row.ParserVersion = parserVersion.String
		}

		var checkedAt sql.NullTime
		var passed sql.NullBool
		var drift []byte
		var canaryParser sql.NullString
		err = s.pool.QueryRow(ctx, `
			SELECT checked_at, passed, drift, parser_version
			FROM contract_canary
			WHERE source = $1
			ORDER BY checked_at DESC
			LIMIT 1`, source,
		).Scan(&checkedAt, &passed, &drift, &canaryParser)
		if err != nil && err != pgx.ErrNoRows {
			return nil, fmt.Errorf("latest canary %s: %w", source, err)
		}
		if err != pgx.ErrNoRows {
			canary := &CanaryHealth{}
			if checkedAt.Valid {
				canary.CheckedAt = &checkedAt.Time
			}
			if passed.Valid {
				canary.Passed = &passed.Bool
			}
			if len(drift) > 0 {
				canary.Drift = json.RawMessage(drift)
			}
			if canaryParser.Valid {
				canary.ParserVersion = canaryParser.String
			}
			row.Canary = canary
		}

		out = append(out, row)
	}
	return out, nil
}

// FeatureCountBaseline returns average successful-poll feature count since the
// provided time. The bool is false when there is no baseline yet.
func (s *Store) FeatureCountBaseline(ctx context.Context, source string, since time.Time) (float64, bool, error) {
	const q = `
		SELECT AVG(feature_count)::float8
		FROM source_polls
		WHERE source = $1
		  AND success = TRUE
		  AND feature_count IS NOT NULL
		  AND started_at >= $2`
	var avg sql.NullFloat64
	if err := s.pool.QueryRow(ctx, q, source, since).Scan(&avg); err != nil {
		return 0, false, fmt.Errorf("feature count baseline %s: %w", source, err)
	}
	if !avg.Valid {
		return 0, false, nil
	}
	return avg.Float64, true, nil
}

// RecordCanary persists one contract canary result.
func (s *Store) RecordCanary(ctx context.Context, result ContractCanaryResult) error {
	const q = `
		INSERT INTO contract_canary (
			source, checked_at, expected_fields, actual_fields, output_sr,
			feature_count, geom_plausible, drift, passed, parser_version
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)`
	var drift any
	if len(result.Drift) > 0 {
		drift = string(result.Drift)
	}
	_, err := s.pool.Exec(ctx, q,
		result.Source, result.CheckedAt, result.ExpectedFields, result.ActualFields, result.OutputSR,
		result.FeatureCount, result.GeomPlausible, drift, result.Passed, result.ParserVersion,
	)
	if err != nil {
		return fmt.Errorf("record canary %s: %w", result.Source, err)
	}
	return nil
}

func (s *Store) RecentCanaryFeatureCounts(ctx context.Context, source string, limit int) ([]int, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(feature_count, 0)
		FROM contract_canary
		WHERE source = $1
		ORDER BY checked_at DESC
		LIMIT $2`, source, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent canary feature counts %s: %w", source, err)
	}
	defer rows.Close()

	counts := []int{}
	for rows.Next() {
		var count int
		if err := rows.Scan(&count); err != nil {
			return nil, err
		}
		counts = append(counts, count)
	}
	return counts, rows.Err()
}

// Ping is a thin wrapper for the API health endpoint.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
