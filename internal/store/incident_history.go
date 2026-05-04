package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

type priorIncident struct {
	NatureCode   string
	NatureDesc   string
	Units        []model.Unit
	Channel      string
	SymbolCode   string
	LocationText string
	Lon          float64
	Lat          float64
	ClearedAt    *time.Time
}

type ClearedIncident struct {
	Source     string
	IncidentID string
	ClearedAt  time.Time
}

func (s *Store) upsertIncidentWithHistory(ctx context.Context, inc model.Incident, pollID int64, observedAt time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin upsert incident: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	prior, exists, err := loadPriorIncident(ctx, tx, inc.Source, inc.IncidentID)
	if err != nil {
		return err
	}

	if err := upsertIncidentSnapshot(ctx, tx, inc, pollID, observedAt); err != nil {
		return err
	}
	if err := syncUnitHistory(ctx, tx, prior, inc, pollID, observedAt); err != nil {
		return err
	}
	if err := writeUpsertEvent(ctx, tx, prior, exists, inc, pollID, observedAt); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit upsert incident: %w", err)
	}
	return nil
}

func loadPriorIncident(ctx context.Context, tx pgx.Tx, source, incidentID string) (*priorIncident, bool, error) {
	const q = `
		SELECT nature_code, nature_desc, units, channel, symbol_code,
		       location_text, ST_X(geom), ST_Y(geom), cleared_at
		FROM incidents
		WHERE source = $1 AND incident_id = $2
		FOR UPDATE`

	var natureCode, natureDesc, channel, symbolCode, locationText sql.NullString
	var lon, lat sql.NullFloat64
	var clearedAt sql.NullTime
	var unitsBytes []byte
	err := tx.QueryRow(ctx, q, source, incidentID).Scan(
		&natureCode, &natureDesc, &unitsBytes, &channel, &symbolCode,
		&locationText, &lon, &lat, &clearedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("load prior incident %s/%s: %w", source, incidentID, err)
	}

	prior := &priorIncident{
		NatureCode:   nullString(natureCode),
		NatureDesc:   nullString(natureDesc),
		Channel:      nullString(channel),
		SymbolCode:   nullString(symbolCode),
		LocationText: nullString(locationText),
		Lon:          nullFloat(lon),
		Lat:          nullFloat(lat),
	}
	if clearedAt.Valid {
		prior.ClearedAt = &clearedAt.Time
	}
	if len(unitsBytes) > 0 {
		if err := json.Unmarshal(unitsBytes, &prior.Units); err != nil {
			return nil, false, fmt.Errorf("decode prior units %s/%s: %w", source, incidentID, err)
		}
	}
	return prior, true, nil
}

func upsertIncidentSnapshot(ctx context.Context, tx pgx.Tx, inc model.Incident, pollID int64, observedAt time.Time) error {
	unitsJSON, err := json.Marshal(inc.Units)
	if err != nil {
		return fmt.Errorf("encode units for %s/%s: %w", inc.Source, inc.IncidentID, err)
	}

	var rawJSON any
	if len(inc.Raw) > 0 {
		rawJSON = string(inc.Raw)
	}

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

	_, err = tx.Exec(ctx, q,
		inc.Source, inc.IncidentID, inc.NatureCode, inc.NatureDesc, string(unitsJSON), inc.Channel,
		inc.SymbolCode, inc.LocationText, inc.Lon, inc.Lat, inc.IncidentDate,
		observedAt, pollID, rawJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert incident %s/%s: %w", inc.Source, inc.IncidentID, err)
	}
	return nil
}

func syncUnitHistory(ctx context.Context, tx pgx.Tx, prior *priorIncident, inc model.Incident, pollID int64, observedAt time.Time) error {
	priorUnits := map[string]model.Unit{}
	if prior != nil {
		priorUnits = unitsByName(prior.Units)
	}

	for _, unit := range inc.Units {
		if unit.Unit == "" {
			continue
		}

		wasPresentInPrior := false
		if _, ok := priorUnits[unit.Unit]; ok {
			wasPresentInPrior = true
		}

		var latestID int64
		var latestStatus string
		err := tx.QueryRow(ctx, `
			SELECT id, status
			FROM incident_units
			WHERE source = $1 AND incident_id = $2 AND unit = $3
			ORDER BY last_observed_at DESC, id DESC
			LIMIT 1`,
			inc.Source, inc.IncidentID, unit.Unit,
		).Scan(&latestID, &latestStatus)

		switch {
		case err == pgx.ErrNoRows:
			if err := insertUnitInterval(ctx, tx, inc, unit, pollID, observedAt); err != nil {
				return err
			}
		case err != nil:
			return fmt.Errorf("load latest unit interval %s/%s/%s: %w", inc.Source, inc.IncidentID, unit.Unit, err)
		case wasPresentInPrior && latestStatus == unit.Status:
			if _, err := tx.Exec(ctx, `
				UPDATE incident_units
				SET last_observed_at = $1, last_poll_id = $2
				WHERE id = $3`,
				observedAt, pollID, latestID,
			); err != nil {
				return fmt.Errorf("extend unit interval %s/%s/%s: %w", inc.Source, inc.IncidentID, unit.Unit, err)
			}
		default:
			if err := insertUnitInterval(ctx, tx, inc, unit, pollID, observedAt); err != nil {
				return err
			}
		}
	}
	return nil
}

func insertUnitInterval(ctx context.Context, tx pgx.Tx, inc model.Incident, unit model.Unit, pollID int64, observedAt time.Time) error {
	const q = `
		INSERT INTO incident_units (
			source, incident_id, unit, status,
			first_observed_at, last_observed_at, first_poll_id, last_poll_id
		) VALUES ($1,$2,$3,$4,$5,$5,$6,$6)`
	_, err := tx.Exec(ctx, q, inc.Source, inc.IncidentID, unit.Unit, unit.Status, observedAt, pollID)
	if err != nil {
		return fmt.Errorf("insert unit interval %s/%s/%s: %w", inc.Source, inc.IncidentID, unit.Unit, err)
	}
	return nil
}

func writeUpsertEvent(ctx context.Context, tx pgx.Tx, prior *priorIncident, exists bool, inc model.Incident, pollID int64, observedAt time.Time) error {
	delta := buildIncidentDelta(prior, inc)

	switch {
	case !exists:
		return writeIncidentEventTx(ctx, tx, inc.Source, inc.IncidentID, "created", observedAt, pollID, delta)
	case prior != nil && prior.ClearedAt != nil:
		return writeIncidentEventTx(ctx, tx, inc.Source, inc.IncidentID, "reopened", observedAt, pollID, delta)
	case !delta.Empty():
		return writeIncidentEventTx(ctx, tx, inc.Source, inc.IncidentID, "updated", observedAt, pollID, delta)
	default:
		return nil
	}
}

func buildIncidentDelta(prior *priorIncident, inc model.Incident) incidentDelta {
	if prior == nil {
		return diffUnits(nil, inc.Units)
	}

	delta := diffUnits(prior.Units, inc.Units)
	appendFieldChange(&delta, "nature_code", prior.NatureCode, inc.NatureCode)
	appendFieldChange(&delta, "nature_desc", prior.NatureDesc, inc.NatureDesc)
	appendFieldChange(&delta, "channel", prior.Channel, inc.Channel)
	appendFieldChange(&delta, "symbol_code", prior.SymbolCode, inc.SymbolCode)
	appendFieldChange(&delta, "location_text", prior.LocationText, inc.LocationText)
	appendFloatFieldChange(&delta, "lon", prior.Lon, inc.Lon)
	appendFloatFieldChange(&delta, "lat", prior.Lat, inc.Lat)
	return delta
}

// RecordClearedEvents appends lifecycle events after SweepCleared marks rows.
func (s *Store) RecordClearedEvents(ctx context.Context, cleared []ClearedIncident, pollID int64) error {
	if len(cleared) == 0 {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin cleared events: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range cleared {
		delta := map[string]time.Time{"cleared_at": item.ClearedAt}
		if err := writeIncidentEventTx(ctx, tx, item.Source, item.IncidentID, "cleared", item.ClearedAt, pollID, delta); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit cleared events: %w", err)
	}
	return nil
}

func writeIncidentEventTx(ctx context.Context, tx pgx.Tx, source, incidentID, eventType string, occurredAt time.Time, pollID int64, delta any) error {
	deltaJSON, err := encodeEventDelta(delta)
	if err != nil {
		return err
	}

	const q = `
		INSERT INTO incident_events (
			source, incident_id, event_type, occurred_at, poll_id, delta
		) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`
	_, err = tx.Exec(ctx, q, source, incidentID, eventType, occurredAt, pollID, deltaJSON)
	if err != nil {
		return fmt.Errorf("insert incident event %s/%s/%s: %w", source, incidentID, eventType, err)
	}
	return nil
}

func encodeEventDelta(delta any) (any, error) {
	if delta == nil {
		return nil, nil
	}
	if d, ok := delta.(incidentDelta); ok && d.Empty() {
		return nil, nil
	}
	b, err := json.Marshal(delta)
	if err != nil {
		return nil, fmt.Errorf("encode event delta: %w", err)
	}
	return string(b), nil
}

func nullString(v sql.NullString) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func nullFloat(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}
