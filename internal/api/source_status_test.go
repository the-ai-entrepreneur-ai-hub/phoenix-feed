package api

import (
	"testing"
	"time"

	"github.com/abusedmindset/phoenix-feed/internal/model"
)

func TestClassifySourceStatusBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name           string
		lastSuccess    *time.Time
		failures       int
		classification string
		reason         string
		unchangedSince *time.Time
		wantStatus     string
		wantReason     string
	}{
		{name: "never succeeded", wantStatus: sourceStatusDown, wantReason: "never_succeeded"},
		{name: "fresh", lastSuccess: timePtr(now.Add(-2*time.Minute - 59*time.Second)), classification: string(model.PollAuthoritativeSuccess), reason: "healthy", wantStatus: sourceStatusOK, wantReason: "healthy"},
		{name: "three minute boundary", lastSuccess: timePtr(now.Add(-3 * time.Minute)), classification: string(model.PollAuthoritativeSuccess), reason: "healthy", wantStatus: sourceStatusDegraded, wantReason: "success_overdue"},
		{name: "one failure", lastSuccess: timePtr(now.Add(-time.Minute)), failures: 1, classification: string(model.PollFailure), reason: "recent_failures", wantStatus: sourceStatusDegraded, wantReason: "recent_failures"},
		{name: "contract failure", lastSuccess: timePtr(now.Add(-time.Minute)), failures: 2, classification: string(model.PollFailure), reason: "contract_invalid", wantStatus: sourceStatusDegraded, wantReason: "contract_invalid"},
		{name: "three failures", lastSuccess: timePtr(now.Add(-time.Minute)), failures: 3, classification: string(model.PollFailure), reason: "recent_failures", wantStatus: sourceStatusDown, wantReason: "recent_failures"},
		{name: "ten minute boundary", lastSuccess: timePtr(now.Add(-10 * time.Minute)), classification: string(model.PollAuthoritativeSuccess), reason: "healthy", wantStatus: sourceStatusDown, wantReason: "success_overdue"},
		{name: "degraded snapshot", lastSuccess: timePtr(now.Add(-time.Minute)), classification: string(model.PollDegradedSnapshot), reason: "snapshot_incomplete", wantStatus: sourceStatusDegraded, wantReason: "snapshot_incomplete"},
		{name: "frozen ten minutes", lastSuccess: timePtr(now.Add(-8 * time.Minute)), classification: string(model.PollDegradedSnapshot), reason: "payload_frozen", unchangedSince: timePtr(now.Add(-10 * time.Minute)), wantStatus: sourceStatusDown, wantReason: "payload_frozen"},
		{name: "clock skew clamps to healthy", lastSuccess: timePtr(now.Add(time.Minute)), classification: string(model.PollAuthoritativeSuccess), reason: "healthy", wantStatus: sourceStatusOK, wantReason: "healthy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotReason := classifySourceStatus(
				now, tt.lastSuccess, tt.failures, tt.classification, tt.reason, tt.unchangedSince,
				3*time.Minute, 10*time.Minute, 3, 10*time.Minute,
			)
			if gotStatus != tt.wantStatus || gotReason != tt.wantReason {
				t.Fatalf("got %s/%s, want %s/%s", gotStatus, gotReason, tt.wantStatus, tt.wantReason)
			}
		})
	}
}

func timePtr(value time.Time) *time.Time { return &value }
