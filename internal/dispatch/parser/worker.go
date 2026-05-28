package dispatchparser

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/geocode"
)

const ParserVersion = "sdr-audio-fallback-phase2"

type Geocoder interface {
	Geocode(context.Context, string) (geocode.Result, error)
}

type Worker struct {
	pool     *pgxpool.Pool
	geocoder Geocoder
	log      *slog.Logger
	now      func() time.Time
}

type WorkerOptions struct {
	Now func() time.Time
}

type BatchStats struct {
	BatchSize               int
	GatePassCount           int
	GateFailCount           int
	GeocodePassCount        int
	GeocodeFailCount        int
	IncidentsInserted       int
	BatchDuration           time.Duration
	LastProcessedTranscript int64
}

type processOutcome int

const (
	outcomeNoRows processOutcome = iota
	outcomeGateFailed
	outcomeGeocodeFailed
	outcomeInserted
)

type transcriptRow struct {
	ID                     int64
	CapturedAt             time.Time
	ReceivedAt             time.Time
	DisplayText            string
	VerificationConfidence float64
}

type incidentUnitJSON struct {
	Unit     string `json:"Unit"`
	Status   string `json:"Status"`
	UnitType string `json:"unit_type"`
	UnitName string `json:"unit_name"`
}

func NewWorker(pool *pgxpool.Pool, geocoder Geocoder, log *slog.Logger, opts WorkerOptions) *Worker {
	if log == nil {
		log = slog.Default()
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Worker{pool: pool, geocoder: geocoder, log: log, now: now}
}

func (w *Worker) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		stats, err := w.ProcessBatch(ctx, 50)
		if err != nil {
			w.log.Error("dispatch parser batch", "err", err)
		} else {
			w.log.Info("dispatch parser batch",
				"batch_size", stats.BatchSize,
				"gate_pass_count", stats.GatePassCount,
				"gate_fail_count", stats.GateFailCount,
				"geocode_pass_count", stats.GeocodePassCount,
				"geocode_fail_count", stats.GeocodeFailCount,
				"incidents_inserted", stats.IncidentsInserted,
				"batch_duration_ms", stats.BatchDuration.Milliseconds(),
			)
		}

		if stats.BatchSize >= 50 {
			continue
		}
		select {
		case <-time.After(interval):
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) ProcessBatch(ctx context.Context, limit int) (BatchStats, error) {
	if limit <= 0 {
		limit = 50
	}
	start := time.Now()
	stats := BatchStats{}
	for stats.BatchSize < limit {
		outcome, transcriptID, err := w.processNext(ctx)
		if err != nil {
			stats.BatchDuration = time.Since(start)
			return stats, err
		}
		if outcome == outcomeNoRows {
			break
		}
		stats.BatchSize++
		stats.LastProcessedTranscript = transcriptID
		switch outcome {
		case outcomeGateFailed:
			stats.GateFailCount++
		case outcomeGeocodeFailed:
			stats.GatePassCount++
			stats.GeocodeFailCount++
		case outcomeInserted:
			stats.GatePassCount++
			stats.GeocodePassCount++
			stats.IncidentsInserted++
		}
	}
	stats.BatchDuration = time.Since(start)
	return stats, nil
}

func (w *Worker) processNext(ctx context.Context) (processOutcome, int64, error) {
	tx, err := w.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return outcomeNoRows, 0, fmt.Errorf("begin dispatch parse tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row, ok, err := lockNextTranscript(ctx, tx)
	if err != nil {
		return outcomeNoRows, 0, err
	}
	if !ok {
		if err := tx.Commit(ctx); err != nil {
			return outcomeNoRows, 0, err
		}
		return outcomeNoRows, 0, nil
	}

	parsed, pass, reason := ParseTranscript(row.DisplayText, row.VerificationConfidence)
	if !pass {
		w.log.Debug("dispatch transcript gate failed", "dispatch_transcript_id", row.ID, "reason", reason)
		if err := markTranscriptParsed(ctx, tx, row.ID, nil); err != nil {
			return outcomeNoRows, row.ID, err
		}
		if err := tx.Commit(ctx); err != nil {
			return outcomeNoRows, row.ID, err
		}
		return outcomeGateFailed, row.ID, nil
	}

	geo, err := w.geocoder.Geocode(ctx, parsed.LocationText)
	if err != nil {
		w.log.Debug("dispatch transcript geocode failed", "dispatch_transcript_id", row.ID, "err", err)
		if err := markTranscriptParsed(ctx, tx, row.ID, nil); err != nil {
			return outcomeNoRows, row.ID, err
		}
		if err := tx.Commit(ctx); err != nil {
			return outcomeNoRows, row.ID, err
		}
		return outcomeGeocodeFailed, row.ID, nil
	}

	incidentDBID, err := w.insertIncident(ctx, tx, row, parsed, geo)
	if err != nil {
		return outcomeNoRows, row.ID, err
	}
	if err := markTranscriptParsed(ctx, tx, row.ID, &incidentDBID); err != nil {
		return outcomeNoRows, row.ID, err
	}
	if err := tx.Commit(ctx); err != nil {
		return outcomeNoRows, row.ID, err
	}
	return outcomeInserted, row.ID, nil
}

func lockNextTranscript(ctx context.Context, tx pgx.Tx) (transcriptRow, bool, error) {
	var row transcriptRow
	var confidence sql.NullFloat64
	err := tx.QueryRow(ctx, `
		SELECT id, captured_at, received_at, display_text, verification_confidence::float8
		FROM dispatch_transcripts
		WHERE parsed_at IS NULL
		ORDER BY received_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED`,
	).Scan(&row.ID, &row.CapturedAt, &row.ReceivedAt, &row.DisplayText, &confidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return transcriptRow{}, false, nil
	}
	if err != nil {
		return transcriptRow{}, false, fmt.Errorf("lock dispatch transcript: %w", err)
	}
	if confidence.Valid {
		row.VerificationConfidence = confidence.Float64
	}
	return row, true, nil
}

func (w *Worker) insertIncident(ctx context.Context, tx pgx.Tx, row transcriptRow, parsed ParsedDispatch, geo geocode.Result) (int64, error) {
	now := w.now().UTC()
	pollID, err := insertParserPoll(ctx, tx, now)
	if err != nil {
		return 0, err
	}
	incidentID := fmt.Sprintf("sdr-%d", row.ID)
	unitsJSON, err := json.Marshal(incidentUnitsJSON(parsed.Units))
	if err != nil {
		return 0, err
	}

	var incidentDBID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO incidents (
			source, incident_id, nature_code, nature_desc, units, channel,
			symbol_code, location_text, geom,
			incident_date, received_at, last_seen_at, last_seen_poll_id
		) VALUES (
			$1,$2,'',$3,$4::jsonb,'',
			'',$5,ST_SetSRID(ST_MakePoint($6,$7),4326),
			$8,$9,$9,$10
		)
		ON CONFLICT (source, incident_id) DO UPDATE SET
			nature_code = EXCLUDED.nature_code,
			nature_desc = EXCLUDED.nature_desc,
			units = EXCLUDED.units,
			channel = EXCLUDED.channel,
			symbol_code = EXCLUDED.symbol_code,
			location_text = EXCLUDED.location_text,
			geom = EXCLUDED.geom,
			incident_date = EXCLUDED.incident_date,
			last_seen_at = EXCLUDED.last_seen_at,
			last_seen_poll_id = EXCLUDED.last_seen_poll_id,
			missing_since = NULL,
			cleared_at = NULL
		RETURNING id`,
		SourceName, incidentID, parsed.Nature, string(unitsJSON), parsed.LocationText, geo.Lon, geo.Lat,
		row.CapturedAt.UTC(), now, pollID,
	).Scan(&incidentDBID)
	if err != nil {
		return 0, fmt.Errorf("insert SDR incident %s: %w", incidentID, err)
	}

	for _, unit := range parsed.Units {
		if _, err := tx.Exec(ctx, `
			INSERT INTO incident_units (
				source, incident_id, unit, status,
				first_observed_at, last_observed_at, first_poll_id, last_poll_id
			) VALUES ($1,$2,$3,$4,$5,$5,$6,$6)
			ON CONFLICT DO NOTHING`,
			SourceName, incidentID, unit.UnitName, "Dispatched", now, pollID,
		); err != nil {
			return 0, fmt.Errorf("insert SDR incident unit %s/%s: %w", incidentID, unit.UnitName, err)
		}
	}
	return incidentDBID, nil
}

func insertParserPoll(ctx context.Context, tx pgx.Tx, now time.Time) (int64, error) {
	var pollID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO source_polls (
			source, request_url, started_at, finished_at, status_code,
			latency_ms, feature_count, parser_version, success
		) VALUES ($1,$2,$3,$3,200,0,1,$4,TRUE)
		RETURNING poll_id`,
		SourceName, "dispatch_transcripts", now, ParserVersion,
	).Scan(&pollID)
	if err != nil {
		return 0, fmt.Errorf("insert SDR source poll: %w", err)
	}
	return pollID, nil
}

func markTranscriptParsed(ctx context.Context, tx pgx.Tx, transcriptID int64, incidentDBID *int64) error {
	_, err := tx.Exec(ctx, `
		UPDATE dispatch_transcripts
		SET parsed_at = NOW(), parsed_incident_id = $2
		WHERE id = $1`,
		transcriptID, incidentDBID,
	)
	if err != nil {
		return fmt.Errorf("mark dispatch transcript parsed %d: %w", transcriptID, err)
	}
	return nil
}

func incidentUnitsJSON(units []ExpectedUnit) []incidentUnitJSON {
	out := make([]incidentUnitJSON, 0, len(units))
	for _, unit := range units {
		out = append(out, incidentUnitJSON{
			Unit:     unit.UnitName,
			Status:   "Dispatched",
			UnitType: unit.UnitType,
			UnitName: unit.UnitName,
		})
	}
	return out
}
