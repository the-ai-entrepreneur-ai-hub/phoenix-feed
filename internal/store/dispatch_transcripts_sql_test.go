package store

import (
	"strings"
	"testing"
)

func TestDispatchTranscriptInsertSQLReturnsDuplicateInSingleConflictStatement(t *testing.T) {
	normalized := compactSQL(insertDispatchTranscriptSQL)
	if !strings.Contains(normalized, "ON CONFLICT (wav_filename) DO UPDATE") {
		t.Fatalf("insert SQL should return duplicate rows through ON CONFLICT DO UPDATE:\n%s", insertDispatchTranscriptSQL)
	}
	for _, banned := range []string{"DO NOTHING", "UNION ALL", "NOT EXISTS"} {
		if strings.Contains(normalized, banned) {
			t.Fatalf("insert SQL should not use %q duplicate fallback:\n%s", banned, insertDispatchTranscriptSQL)
		}
	}
}

func TestListRecentDispatchTranscriptsSQLOrdersByReceivedAtForIndex(t *testing.T) {
	normalized := compactSQL(listRecentDispatchTranscriptsSQL)
	if !strings.Contains(normalized, "ORDER BY received_at DESC LIMIT $1") {
		t.Fatalf("recent SQL should match idx_dispatch_transcripts_received_at order:\n%s", listRecentDispatchTranscriptsSQL)
	}
	if strings.Contains(normalized, "id DESC") {
		t.Fatalf("recent SQL should not add a secondary sort that can defeat the received_at index:\n%s", listRecentDispatchTranscriptsSQL)
	}
}

func TestDispatchTranscriptHealthSQLUsesReceivedAtWindows(t *testing.T) {
	normalized := compactSQL(dispatchTranscriptHealthSQL)
	for _, want := range []string{
		"ORDER BY received_at DESC LIMIT 1",
		"WHERE received_at >= $1",
		"WHERE received_at >= $2",
		"review_recommended = TRUE",
	} {
		if !strings.Contains(normalized, want) {
			t.Fatalf("health SQL missing %q:\n%s", want, dispatchTranscriptHealthSQL)
		}
	}
}

func compactSQL(sql string) string {
	return strings.Join(strings.Fields(sql), " ")
}
