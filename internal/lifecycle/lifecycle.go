// Package lifecycle owns the rules for how an incident transitions between
// observed → missing → cleared → reopened, and is the only place those
// rules live. The ingester delegates here after every poll.
package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

type Manager struct {
	store            *store.Store
	clearAfterMisses int
	log              *slog.Logger
}

func New(s *store.Store, clearAfterMisses int, log *slog.Logger) *Manager {
	return &Manager{store: s, clearAfterMisses: clearAfterMisses, log: log}
}

// Apply records one poll result and runs the lifecycle state machine.
//
// On a successful poll we:
//  1. Insert a row in source_polls and capture poll_id.
//  2. Upsert each observed incident (resets missing_since, may bump reopen_count).
//  3. Mark non-observed incidents as missing (idempotent).
//  4. Sweep incidents that have hit the consecutive-miss threshold to cleared.
//
// On a failed poll we ONLY record the source_polls row. Failed polls never
// advance the clearing state — that was the whole point of the Codex fix.
func (m *Manager) Apply(ctx context.Context, r model.PollResult) error {
	pollID, err := m.store.RecordPoll(ctx, r)
	if err != nil {
		return err
	}

	if !r.Success() {
		m.log.Warn("poll failed",
			"source", r.Source,
			"status", r.StatusCode,
			"err", errString(r.Err),
			"poll_id", pollID)
		return nil
	}

	observedAt := r.StartedAt

	observedIDs := make([]string, 0, len(r.Incidents))
	for _, inc := range r.Incidents {
		if err := m.store.UpsertIncident(ctx, inc, pollID, observedAt); err != nil {
			m.log.Error("upsert", "incident_id", inc.IncidentID, "err", err)
			continue
		}
		observedIDs = append(observedIDs, inc.IncidentID)
	}

	if err := m.store.MarkMissing(ctx, r.Source, observedAt, observedIDs); err != nil {
		m.log.Error("mark missing", "err", err)
	}

	cleared, err := m.store.SweepCleared(ctx, r.Source, m.clearAfterMisses)
	if err != nil {
		m.log.Error("sweep cleared", "err", err)
	} else if len(cleared) > 0 {
		if err := m.store.RecordClearedEvents(ctx, cleared, pollID); err != nil {
			m.log.Error("record cleared events", "err", err)
		}
		m.log.Info("incidents cleared", "source", r.Source, "count", len(cleared), "ids", clearedIncidentIDs(cleared))
	}

	m.log.Info("poll applied",
		"source", r.Source,
		"observed", len(observedIDs),
		"cleared", len(cleared),
		"latency_ms", r.LatencyMS,
		"poll_id", pollID)
	return nil
}

func clearedIncidentIDs(cleared []store.ClearedIncident) []string {
	ids := make([]string, 0, len(cleared))
	for _, item := range cleared {
		ids = append(ids, item.IncidentID)
	}
	return ids
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Jitter returns a duration uniformly distributed in [base-jitter, base+jitter].
// Exported so callers in cmd/ingester can use it with their own RNG.
func Jitter(base, jitter time.Duration, rnd func() float64) time.Duration {
	if jitter <= 0 {
		return base
	}
	delta := time.Duration((rnd()*2 - 1) * float64(jitter))
	return base + delta
}
