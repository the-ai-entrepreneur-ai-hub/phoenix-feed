package api

import (
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
	"github.com/abusedmindset/phoenix-feed/internal/store"
)

const (
	sourceStatusOK       = "ok"
	sourceStatusDegraded = "degraded"
	sourceStatusDown     = "down"
)

func applySourceStatus(meta *store.StalenessMeta, now time.Time, cfg Config) {
	degradedAfter, downAfter, downFailures, frozenDownAfter := sourceThresholds(cfg)
	status, reason := classifySourceStatus(
		now.UTC(), meta.SourceLastSuccessAt, meta.SourceConsecutiveFailures,
		meta.LatestPollClassification, meta.LatestPollReason, meta.LatestUnchangedSince,
		degradedAfter, downAfter, downFailures, frozenDownAfter,
	)
	meta.SourceStatus = status
	meta.SourceStatusReason = reason
	if meta.SourceLastSuccessAt == nil {
		meta.DataAgeSeconds = nil
	} else if meta.DataAgeSeconds == nil {
		age := elapsedSeconds(now, *meta.SourceLastSuccessAt)
		meta.DataAgeSeconds = &age
	}
}

func classifySourceStatus(
	now time.Time,
	lastSuccess *time.Time,
	consecutiveFailures int,
	latestClassification, latestReason string,
	unchangedSince *time.Time,
	degradedAfter, downAfter time.Duration,
	downFailures int,
	frozenDownAfter time.Duration,
) (string, string) {
	if lastSuccess == nil {
		return sourceStatusDown, "never_succeeded"
	}
	if latestReason == "payload_frozen" && unchangedSince != nil && now.Sub(*unchangedSince) >= frozenDownAfter {
		return sourceStatusDown, "payload_frozen"
	}
	if consecutiveFailures >= downFailures {
		return sourceStatusDown, stableSourceReason(latestReason, "recent_failures")
	}
	age := now.Sub(*lastSuccess)
	if age >= downAfter {
		if latestReason == "payload_frozen" {
			return sourceStatusDown, "payload_frozen"
		}
		return sourceStatusDown, "success_overdue"
	}
	if latestClassification == string(model.PollDegradedSnapshot) {
		return sourceStatusDegraded, stableSourceReason(latestReason, "snapshot_incomplete")
	}
	if consecutiveFailures > 0 || latestClassification == string(model.PollFailure) {
		return sourceStatusDegraded, stableSourceReason(latestReason, "recent_failures")
	}
	if age >= degradedAfter {
		return sourceStatusDegraded, "success_overdue"
	}
	return sourceStatusOK, "healthy"
}

func stableSourceReason(reason, fallback string) string {
	switch reason {
	case "healthy", "recent_failures", "success_overdue", "contract_invalid", "snapshot_incomplete", "payload_frozen", "never_succeeded":
		return reason
	default:
		return fallback
	}
}

func sourceThresholds(cfg Config) (time.Duration, time.Duration, int, time.Duration) {
	degradedAfter := cfg.SourceDegradedAfter
	if degradedAfter <= 0 {
		degradedAfter = 3 * time.Minute
	}
	downAfter := cfg.SourceDownAfter
	if downAfter <= 0 {
		downAfter = cfg.StaleAfter
		if downAfter <= 0 {
			downAfter = 10 * time.Minute
		}
	}
	downFailures := cfg.SourceDownFailures
	if downFailures <= 0 {
		downFailures = 3
	}
	frozenDownAfter := cfg.FrozenDownAfter
	if frozenDownAfter <= 0 {
		frozenDownAfter = 10 * time.Minute
	}
	return degradedAfter, downAfter, downFailures, frozenDownAfter
}

func elapsedSeconds(now, then time.Time) int {
	seconds := int(now.UTC().Sub(then.UTC()).Seconds())
	if seconds < 0 {
		return 0
	}
	return seconds
}

func applyIncidentAuthority(meta store.StalenessMeta, incidents []store.ActiveIncident) {
	for i := range incidents {
		incidents[i].LastConfirmedAt = incidents[i].LastSeenAt
		incidents[i].IsLastKnown = incidentIsLastKnown(meta, incidents[i].LastSeenAt)
	}
}

func incidentIsLastKnown(meta store.StalenessMeta, lastConfirmed time.Time) bool {
	if meta.SourceStatus != sourceStatusOK || meta.SourceLastSuccessAt == nil {
		return true
	}
	return lastConfirmed.Before(meta.SourceLastSuccessAt.UTC())
}
