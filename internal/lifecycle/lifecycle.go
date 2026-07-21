// Package lifecycle owns the rules for how an incident transitions between
// observed → missing → cleared → reopened, and is the only place those
// rules live. The ingester delegates here after every poll.
package lifecycle

import (
	"context"
	"fmt"
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
// On a failed or degraded poll we ONLY record the source_polls row.
// Non-authoritative polls never advance the clearing state.
func (m *Manager) Apply(ctx context.Context, r model.PollResult) error {
	if !r.Success() {
		pollID, err := m.store.RecordPoll(ctx, r)
		if err != nil {
			return err
		}
		m.log.Warn("poll not authoritative",
			"source", r.Source,
			"status", r.StatusCode,
			"classification", r.Classification,
			"reason", r.Reason,
			"err", errString(r.Err),
			"poll_id", pollID)
		return nil
	}

	pollID, err := m.store.RecordPollPending(ctx, r)
	if err != nil {
		return err
	}
	observedAt := r.StartedAt

	observedIDs := make([]string, 0, len(r.Incidents))
	for _, inc := range r.Incidents {
		if err := m.store.UpsertIncident(ctx, inc, pollID, observedAt); err != nil {
			applyErr := fmt.Errorf("upsert incident %s/%s: %w", inc.Source, inc.IncidentID, err)
			m.log.Error("upsert", "incident_id", inc.IncidentID, "err", err)
			return m.failPollApply(ctx, pollID, applyErr)
		}
		observedIDs = append(observedIDs, inc.IncidentID)
	}

	if err := m.store.MarkMissing(ctx, r.Source, observedAt, observedIDs); err != nil {
		applyErr := fmt.Errorf("mark missing %s: %w", r.Source, err)
		m.log.Error("mark missing", "err", err)
		return m.failPollApply(ctx, pollID, applyErr)
	}

	cleared, err := m.store.SweepCleared(ctx, r.Source, m.clearAfterMisses)
	if err != nil {
		applyErr := fmt.Errorf("sweep cleared %s: %w", r.Source, err)
		m.log.Error("sweep cleared", "err", err)
		return m.failPollApply(ctx, pollID, applyErr)
	} else if len(cleared) > 0 {
		if err := m.store.RecordClearedEvents(ctx, cleared, pollID); err != nil {
			applyErr := fmt.Errorf("record cleared events %s: %w", r.Source, err)
			m.log.Error("record cleared events", "err", err)
			return m.failPollApply(ctx, pollID, applyErr)
		}
		m.log.Info("incidents cleared", "source", r.Source, "count", len(cleared), "ids", clearedIncidentIDs(cleared))
	}

	if err := m.store.MarkPollSucceeded(ctx, pollID); err != nil {
		return fmt.Errorf("mark poll succeeded %d: %w", pollID, err)
	}

	m.log.Info("poll applied",
		"source", r.Source,
		"observed", len(observedIDs),
		"cleared", len(cleared),
		"latency_ms", r.LatencyMS,
		"poll_id", pollID)
	return nil
}

func (m *Manager) failPollApply(ctx context.Context, pollID int64, applyErr error) error {
	if err := m.store.MarkPollFailed(ctx, pollID, applyErr); err != nil {
		return fmt.Errorf("%w; additionally failed to mark poll failed: %v", applyErr, err)
	}
	return applyErr
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
