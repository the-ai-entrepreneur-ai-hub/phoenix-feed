package store

import (
	"os"
	"strings"
	"testing"
)

func TestActiveIncidentsViewHasNoDispatchAgeCutoff(t *testing.T) {
	schema, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(schema)
	viewStart := strings.Index(sql, "CREATE OR REPLACE VIEW active_incidents AS")
	if viewStart == -1 {
		t.Fatal("active_incidents view not found")
	}
	viewSQL := sql[viewStart:]

	if !strings.Contains(viewSQL, "WHERE i.cleared_at IS NULL") {
		t.Fatal("active_incidents view must be keyed to cleared_at, not dispatch age")
	}
	for _, banned := range []string{"incident_date <", "received_at <", "90 minutes", "INTERVAL '90"} {
		if strings.Contains(viewSQL, banned) {
			t.Fatalf("active_incidents view contains dispatch-age cutoff %q", banned)
		}
	}
}
