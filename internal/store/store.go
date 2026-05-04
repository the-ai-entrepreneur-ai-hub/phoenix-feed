// Package store wraps Postgres access. No ORM. Hand-written SQL keeps the
// schema visible at the call site, which matters more than the boilerplate
// it costs.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/model"
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

type StalenessMeta struct {
	SourceLastSuccessAt *time.Time `json:"source_last_success_at"`
	DataAgeSeconds      *int       `json:"data_age_seconds"`
	ParserVersion       string     `json:"parser_version"`
}

type ActiveIncident struct {
	Source               string       `json:"source"`
	IncidentID           string       `json:"incident_id"`
	NatureCode           string       `json:"nature_code,omitempty"`
	NatureDesc           string       `json:"nature_desc,omitempty"`
	Units                []model.Unit `json:"units,omitempty"`
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

func (s *Store) Close() { s.pool.Close() }

// RecordPoll inserts a row into source_polls and returns the new poll_id.
// Called both on success and failure so the audit trail is complete.
func (s *Store) RecordPoll(ctx context.Context, r model.PollResult) (int64, error) {
	const q = `
		INSERT INTO source_polls (
			source, request_url, started_at, finished_at, status_code,
			latency_ms, feature_count, payload_sha256, parser_version,
			success, error
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),$9,$10,NULLIF($11,''))
		RETURNING poll_id`

	var errStr string
	if r.Err != nil {
		errStr = r.Err.Error()
	}

	var id int64
	err := s.pool.QueryRow(ctx, q,
		r.Source, r.RequestURL, r.StartedAt, r.FinishedAt, r.StatusCode,
		r.LatencyMS, len(r.Incidents), r.PayloadSHA256, r.ParserVersion,
		r.Success(), errStr,
	).Scan(&id)
	return id, err
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
		var inc ActiveIncident
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
		}
		incidents = append(incidents, inc)
	}
	if err := rows.Err(); err != nil {
		return ActiveIncidentsResult{}, err
	}

	return ActiveIncidentsResult{Meta: meta, Incidents: incidents}, nil
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

// UpsertIncident applies a single observed incident, recording the
// (source, incident_id) row and resetting any clearing state. Called by the
// lifecycle layer for each feature observed in a successful poll.
func (s *Store) UpsertIncident(ctx context.Context, inc model.Incident, pollID int64, observedAt time.Time) error {
	unitsJSON, _ := json.Marshal(inc.Units)

	const q = `
		INSERT INTO incidents (
			source, incident_id, nature_code, nature_desc, units, channel,
			symbol_code, location_text, geom, incident_date,
			received_at, last_seen_at, last_seen_poll_id, raw
		) VALUES (
			$1,$2,$3,$4,$5::jsonb,$6,
			$7,$8, ST_SetSRID(ST_MakePoint($9,$10),4326), $11,
			$12,$12,$13,$14::jsonb
		)
		ON CONFLICT (source, incident_id) DO UPDATE SET
			nature_code       = EXCLUDED.nature_code,
			nature_desc       = EXCLUDED.nature_desc,
			units             = EXCLUDED.units,
			channel           = EXCLUDED.channel,
			symbol_code       = EXCLUDED.symbol_code,
			location_text     = EXCLUDED.location_text,
			geom              = EXCLUDED.geom,
			incident_date     = EXCLUDED.incident_date,
			last_seen_at      = EXCLUDED.last_seen_at,
			last_seen_poll_id = EXCLUDED.last_seen_poll_id,
			missing_since     = NULL,
			cleared_at        = NULL,
			reopen_count      = CASE
				WHEN incidents.cleared_at IS NOT NULL THEN incidents.reopen_count + 1
				ELSE incidents.reopen_count
			END,
			raw               = COALESCE(EXCLUDED.raw, incidents.raw)`

	_, err := s.pool.Exec(ctx, q,
		inc.Source, inc.IncidentID, inc.NatureCode, inc.NatureDesc, string(unitsJSON), inc.Channel,
		inc.SymbolCode, inc.LocationText, inc.Lon, inc.Lat, inc.IncidentDate,
		observedAt, pollID, string(inc.Raw),
	)
	return err
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
// at least minMisses consecutive successful polls. Returns the IDs of cleared
// incidents so the caller can emit incident_events.
func (s *Store) SweepCleared(ctx context.Context, source string, minMisses int) ([]string, error) {
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
		RETURNING i.incident_id`

	rows, err := s.pool.Query(ctx, q, source, minMisses)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
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

// Ping is a thin wrapper for the API health endpoint.
func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
