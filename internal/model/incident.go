// Package model contains the cross-package types that flow from source
// adapters through the lifecycle layer into the store.
package model

import "time"

// Incident is the parser-normalized view of a single feature from any source.
// Source-specific quirks (HTML entities, weird whitespace, native projections)
// are resolved before this struct is constructed.
type Incident struct {
	Source       string    // e.g. "phoenix-fire-mapserver"
	IncidentID   string    // upstream stable ID, e.g. "F26198635"
	NatureCode   string    // short code, e.g. "WF"
	NatureDesc   string    // human-readable, trimmed
	Units        []Unit    // current snapshot of units + status
	Channel      string
	SymbolCode   string
	LocationText string
	Lon          float64   // WGS84
	Lat          float64   // WGS84
	IncidentDate time.Time // from feed
	Raw          []byte    // original feature JSON, retained per retention policy
}

// Unit represents one apparatus and its observed status on this incident.
type Unit struct {
	Unit   string // e.g. "E2203"
	Status string // e.g. "On Scene", "Dispatched", "Available"
}

// PollResult is what a Source returns from one fetch attempt.
type PollResult struct {
	Source        string
	RequestURL    string
	StartedAt     time.Time
	FinishedAt    time.Time
	StatusCode    int
	LatencyMS     int
	Incidents     []Incident
	PayloadSHA256 string
	ParserVersion string
	Err           error // non-nil on transport / parse failure
}

// Success returns true when the poll round-tripped cleanly and the parser
// produced a feature set the caller can trust. Only successful polls count
// toward the lifecycle clearing rule.
func (r PollResult) Success() bool {
	return r.Err == nil && r.StatusCode >= 200 && r.StatusCode < 300
}
