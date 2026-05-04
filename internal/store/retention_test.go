package store

import (
	"testing"
	"time"
)

func TestRawRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	got := RawRetentionCutoff(now, 30*24*time.Hour)

	want := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("cutoff = %s, want %s", got, want)
	}
}

func TestRawRetentionCutoffRejectsNonPositiveRetention(t *testing.T) {
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

	got := RawRetentionCutoff(now, 0)

	if !got.Equal(now) {
		t.Fatalf("cutoff = %s, want now %s", got, now)
	}
}
