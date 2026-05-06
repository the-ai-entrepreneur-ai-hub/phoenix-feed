package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/abusedmindset/phoenix-feed/internal/backfill"
	"github.com/abusedmindset/phoenix-feed/internal/config"
	"github.com/abusedmindset/phoenix-feed/internal/model"
)

type incidentRow struct {
	source       string
	incidentID   string
	raw          []byte
	units        []byte
	receivedAt   time.Time
	lastSeenPoll sql.NullInt64
}

type summary struct {
	rowsProcessed      int
	rowsUpdated        int
	beforeSmashed      int
	afterSmashed       int
	flippedToMultiUnit int
	elapsed            time.Duration
}

func main() {
	started := time.Now()
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fatal("config", err)
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("pgx pool", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("pg ping", err)
	}

	result, err := run(ctx, pool)
	if err != nil {
		fatal("backfill units", err)
	}
	result.elapsed = time.Since(started)
	fmt.Printf("backfill_units complete rows_processed=%d rows_updated=%d before_smashed=%d after_smashed=%d flipped_to_multi_unit=%d elapsed=%s\n",
		result.rowsProcessed,
		result.rowsUpdated,
		result.beforeSmashed,
		result.afterSmashed,
		result.flippedToMultiUnit,
		result.elapsed.Round(time.Millisecond),
	)
}

func run(ctx context.Context, pool *pgxpool.Pool) (summary, error) {
	rows, err := pool.Query(ctx, `
		SELECT source, incident_id, raw, units, received_at, last_seen_poll_id
		FROM incidents
		WHERE units IS NOT NULL
		  AND raw IS NOT NULL
		  AND raw -> 'attributes' ? 'Units'
		ORDER BY received_at, incident_id`)
	if err != nil {
		return summary{}, fmt.Errorf("query incidents: %w", err)
	}
	defer rows.Close()

	candidates := []incidentRow{}
	for rows.Next() {
		var row incidentRow
		if err := rows.Scan(&row.source, &row.incidentID, &row.raw, &row.units, &row.receivedAt, &row.lastSeenPoll); err != nil {
			return summary{}, err
		}
		candidates = append(candidates, row)
	}
	if err := rows.Err(); err != nil {
		return summary{}, err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return summary{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var out summary
	for _, row := range candidates {
		out.rowsProcessed++

		var prior []model.Unit
		if len(row.units) > 0 {
			if err := json.Unmarshal(row.units, &prior); err != nil {
				return summary{}, fmt.Errorf("decode prior units %s/%s: %w", row.source, row.incidentID, err)
			}
		}
		beforeSmashed := backfill.HasSmashedStatus(prior)
		if beforeSmashed {
			out.beforeSmashed++
		}

		parsed, hasUnits, err := backfill.UnitsFromRaw(row.raw)
		if err != nil {
			return summary{}, fmt.Errorf("parse raw units %s/%s: %w", row.source, row.incidentID, err)
		}
		if !hasUnits {
			continue
		}
		afterSmashed := backfill.HasSmashedStatus(parsed)
		if afterSmashed {
			out.afterSmashed++
		}
		if beforeSmashed && !afterSmashed && len(parsed) > 1 {
			out.flippedToMultiUnit++
		}

		unitsJSON, err := json.Marshal(parsed)
		if err != nil {
			return summary{}, fmt.Errorf("encode parsed units %s/%s: %w", row.source, row.incidentID, err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE incidents
			SET units = $3::jsonb
			WHERE source = $1 AND incident_id = $2`,
			row.source, row.incidentID, string(unitsJSON),
		)
		if err != nil {
			return summary{}, fmt.Errorf("update incident %s/%s: %w", row.source, row.incidentID, err)
		}
		out.rowsUpdated += int(tag.RowsAffected())

		if _, err := tx.Exec(ctx, `
			DELETE FROM incident_units
			WHERE source = $1 AND incident_id = $2`,
			row.source, row.incidentID,
		); err != nil {
			return summary{}, fmt.Errorf("delete unit history %s/%s: %w", row.source, row.incidentID, err)
		}

		if len(parsed) > 0 && !row.lastSeenPoll.Valid {
			return summary{}, fmt.Errorf("missing last_seen_poll_id for %s/%s", row.source, row.incidentID)
		}
		for _, unit := range parsed {
			if unit.Unit == "" {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO incident_units (
					source, incident_id, unit, status,
					first_observed_at, last_observed_at, first_poll_id, last_poll_id
				) VALUES ($1,$2,$3,$4,$5,$5,$6,$6)`,
				row.source, row.incidentID, unit.Unit, unit.Status, row.receivedAt, row.lastSeenPoll.Int64,
			); err != nil {
				return summary{}, fmt.Errorf("insert unit history %s/%s/%s: %w", row.source, row.incidentID, unit.Unit, err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return summary{}, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

func fatal(msg string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	os.Exit(1)
}
