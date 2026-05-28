package dispatchparser

import (
	"regexp"
	"strings"
)

const SourceName = "sdr_audio"

var (
	cdecPattern = regexp.MustCompile(`(?i)\b(?:CDEC|Sea[\s-]?Deck|Sea[\s-]?Beck|Seabex|FedEx|CDC)[\s-]?(\d+)\b`)
	unitPattern = regexp.MustCompile(`(?i)\b(engine|ladder|battalion|rescue|squad|truck|medic|chief|amr)[\s-]?(\d+)\b`)

	addressPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b\d+(?:-\d+)?\s+(?:north|south|east|west|n|s|e|w)\s+[a-z0-9]+(?:[\s-]+[a-z0-9]+){0,5}\s+(?:avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)\b`),
		regexp.MustCompile(`(?i)\b(?:north|south|east|west|n|s|e|w)\s+[a-z0-9]+(?:[\s-]+[a-z0-9]+){0,5}\s+(?:avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)\s+(?:and|at|/)\s+(?:north|south|east|west|n|s|e|w)\s+[a-z0-9]+(?:[\s-]+[a-z0-9]+){0,5}\s+(?:avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)\b`),
		regexp.MustCompile(`(?i)\b(?:north|south|east|west|n|s|e|w)\s+\d+[a-z]{0,2}\s+(?:avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)\b`),
	}
)

type ExpectedUnit struct {
	UnitName string `json:"unit_name"`
	UnitType string `json:"unit_type"`
}

type ParsedDispatch struct {
	Nature       string
	Channel      string
	Units        []ExpectedUnit
	LocationText string
}

func ParseTranscript(text string, confidence float64) (ParsedDispatch, bool, string) {
	text = strings.TrimSpace(text)
	if confidence < 0.80 {
		return ParsedDispatch{}, false, "low_confidence"
	}
	channel := extractCDECChannel(text)
	if channel == "" {
		return ParsedDispatch{}, false, "missing_cdec"
	}
	units := extractUnits(text)
	if len(units) == 0 {
		return ParsedDispatch{}, false, "missing_unit"
	}
	address, addressStart, ok := findAddress(text)
	if !ok {
		return ParsedDispatch{}, false, "missing_address"
	}
	nature := extractNature(text[:addressStart])
	if nature == "" {
		return ParsedDispatch{}, false, "missing_nature"
	}
	return ParsedDispatch{Nature: nature, Channel: channel, Units: units, LocationText: address}, true, ""
}

func extractCDECChannel(text string) string {
	match := cdecPattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return "CDEC " + match[1]
}

func extractUnits(text string) []ExpectedUnit {
	matches := unitPattern.FindAllStringSubmatch(text, -1)
	out := []ExpectedUnit{}
	seen := map[string]bool{}
	for _, match := range matches {
		unitType := canonicalUnitType(match[1])
		unitName := unitType + " " + match[2]
		key := strings.ToLower(unitName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ExpectedUnit{UnitName: unitName, UnitType: unitType})
	}
	return out
}

func findAddress(text string) (string, int, bool) {
	bestStart := -1
	bestEnd := -1
	for _, pattern := range addressPatterns {
		loc := pattern.FindStringIndex(text)
		if loc == nil {
			continue
		}
		if bestStart == -1 || loc[0] < bestStart {
			bestStart = loc[0]
			bestEnd = loc[1]
		}
	}
	if bestStart == -1 {
		return "", 0, false
	}
	return strings.Trim(strings.TrimSpace(text[bestStart:bestEnd]), ".,"), bestStart, true
}

func extractNature(prefix string) string {
	markers := cdecPattern.FindAllStringIndex(prefix, -1)
	if len(markers) == 0 {
		return ""
	}
	candidate := prefix[markers[len(markers)-1][1]:]
	candidate = strings.Trim(candidate, " \t\r\n.,;:-")
	for {
		loc := cdecPattern.FindStringIndex(candidate)
		if loc == nil || loc[0] != 0 {
			break
		}
		candidate = strings.Trim(candidate[loc[1]:], " \t\r\n.,;:-")
	}
	if idx := strings.IndexAny(candidate, ".;"); idx >= 0 {
		candidate = candidate[:idx]
	}
	candidate = strings.Trim(candidate, " \t\r\n.,;:-")
	if candidate == "" {
		return ""
	}
	if nature := matchKnownNature(candidate); nature != "" {
		return nature
	}
	return "Dispatch Call"
}

func canonicalUnitType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "amr" {
		return "AMR"
	}
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
