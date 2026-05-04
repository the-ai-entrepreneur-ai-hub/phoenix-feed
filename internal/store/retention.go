package store

import (
	"context"
	"fmt"
	"time"
)

// RawRetentionCutoff returns the oldest last_seen_at that may retain raw JSON.
func RawRetentionCutoff(now time.Time, retention time.Duration) time.Time {
	if retention <= 0 {
		return now
	}
	return now.Add(-retention)
}

// DropRawOlderThan drops raw JSONB for incidents older than the configured
// retention while preserving normalized incident fields.
func (s *Store) DropRawOlderThan(ctx context.Context, now time.Time, retention time.Duration) (int64, error) {
	cutoff := RawRetentionCutoff(now, retention)
	tag, err := s.pool.Exec(ctx, `
		UPDATE incidents
		SET raw = NULL,
		    raw_dropped_at = $1
		WHERE raw IS NOT NULL
		  AND raw_dropped_at IS NULL
		  AND last_seen_at < $2`, now, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("drop raw older than %s: %w", cutoff.Format(time.RFC3339), err)
	}
	return tag.RowsAffected(), nil
}

// VacuumAnalyzeIncidents refreshes storage statistics after retention sweeps.
func (s *Store) VacuumAnalyzeIncidents(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `VACUUM ANALYZE incidents`); err != nil {
		return fmt.Errorf("vacuum analyze incidents: %w", err)
	}
	return nil
}
