package dispatchparser

import (
	"regexp"
	"sort"
	"strings"
)

const SourceName = "sdr_audio"

var (
	cdecPattern          = regexp.MustCompile(`(?i)\b(?:CDEC|Sea[\s-]?Deck|Sea[\s-]?Beck|Seabex|FedEx|CDC|K[\s-]?Deck|Cadeck)[\s-]?(\d+)\b`)
	unitPattern          = regexp.MustCompile(`(?i)\b(engine|ladder|battalion|rescue|squad|truck|medic|chief|amr)[\s-]?(\d+)\b`)
	hyphenatedAMRPattern = regexp.MustCompile(`(?i)\bA[\s.-]*M[\s.-]*R[\s.-]*(\d(?:[\s.-]*\d)+)\b`)

	directionPattern    = `(?:north|south|east|west|n|s|e|w|e[\s-]?suite)`
	streetSuffixPattern = `(?:avenue|street|road|drive|place|boulevard|way|court|lane|ave|st|rd|dr|pl|blvd)`
	streetNamePattern   = `[a-z0-9]+(?:[\s-]+[a-z0-9]+){0,5}`

	addressPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b\d+(?:[-\s]\d+)?[\s,.]+` + directionPattern + `\s+` + streetNamePattern + `\s+` + streetSuffixPattern + `\b`),
		regexp.MustCompile(`(?i)\b` + directionPattern + `\s+` + streetNamePattern + `\s+` + streetSuffixPattern + `\s+(?:and|at|/)\s+` + directionPattern + `\s+` + streetNamePattern + `\s+` + streetSuffixPattern + `\b`),
		regexp.MustCompile(`(?i)\b` + streetNamePattern + `\s+` + streetSuffixPattern + `\s+(?:and|at|/)\s+` + streetNamePattern + `\s+` + streetSuffixPattern + `\b`),
		regexp.MustCompile(`(?i)\b` + directionPattern + `\s+\d+[a-z]{0,2}\s+` + streetSuffixPattern + `\b`),
		regexp.MustCompile(`(?i)\bSignal\s+Butte(?:\s+(?:Road|Rd))?\s+(?:and|at|/)\s+\d+(?:\s+over\s+under)?\b`),
		regexp.MustCompile(`(?i)\b(?:I|Interstate|US|SR|State\s+Route|Loop)\s*-?\s*\d+(?:\s+(?:and|at|/)\s+` + streetNamePattern + `\s+` + streetSuffixPattern + `)?\b`),
	}

	whitespacePattern                 = regexp.MustCompile(`\s+`)
	punctuatedDirectionPattern        = regexp.MustCompile(`(?i)\b(\d+(?:-\d+)?)\s*[,\.]\s*(north|south|east|west|n|s|e|w)\b`)
	splitHouseNumberPattern           = regexp.MustCompile(`(?i)\b(\d{2,5})\s+(\d{2,4})\s+(north|south|east|west|n|s|e|w)\b`)
	esuiteDirectionPattern            = regexp.MustCompile(`(?i)\be[\s-]?suite\b`)
	overUnderSuffixPattern            = regexp.MustCompile(`(?i)\s+over\s+under$`)
	leadingNatureAddressNoisePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^working\s+fire\s+`),
		regexp.MustCompile(`(?i)^house\s+fire\s+`),
		regexp.MustCompile(`(?i)^structure\s+fire\s+`),
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
	channelLoc := cdecPattern.FindStringSubmatchIndex(text)
	if len(channelLoc) < 4 {
		return ParsedDispatch{}, false, "missing_cdec"
	}
	if parsed, ok, reason := parseChannelFirstSegment(text, channelLoc); ok {
		return parsed, true, ""
	} else if reason != "missing_address" {
		return ParsedDispatch{}, false, reason
	}
	if parsed, ok, reason := parseAddressBeforeChannelSegment(text, channelLoc); ok {
		return parsed, true, ""
	} else {
		return ParsedDispatch{}, false, reason
	}
}

func parseChannelFirstSegment(text string, channelLoc []int) (ParsedDispatch, bool, string) {
	channelStart := channelLoc[0]
	channelEnd := channelSequenceEnd(text, channelLoc[1])
	address, addressStart, addressEnd, ok := findAddressAfter(text, channelEnd)
	if !ok {
		return ParsedDispatch{}, false, "missing_address"
	}
	if nextChannel := nextCDECMarkerStart(text, channelEnd); nextChannel >= 0 && nextChannel < addressStart {
		return ParsedDispatch{}, false, "missing_address"
	}

	segmentStart := dispatchSegmentStart(text, channelStart)
	segmentEnd := addressEnd
	segment := text[segmentStart:segmentEnd]
	relativeAddressStart := addressStart - segmentStart
	units := extractUnits(segment)
	if len(units) == 0 {
		extendedEnd := extendSegmentToTrailingUnit(text, segmentEnd)
		if extendedEnd > segmentEnd {
			segmentEnd = extendedEnd
			segment = text[segmentStart:segmentEnd]
			units = extractUnits(segment)
		}
	}
	if len(units) == 0 {
		return ParsedDispatch{}, false, "missing_unit"
	}
	nature := extractNature(segment[:relativeAddressStart])
	if nature == "" {
		return ParsedDispatch{}, false, "missing_nature"
	}
	channel := extractCDECChannel(text[channelStart:channelEnd])
	return ParsedDispatch{Nature: nature, Channel: channel, Units: units, LocationText: address}, true, ""
}

func parseAddressBeforeChannelSegment(text string, channelLoc []int) (ParsedDispatch, bool, string) {
	channelStart := channelLoc[0]
	channelEnd := channelSequenceEnd(text, channelLoc[1])
	address, addressStart, addressEnd, ok := findAddressBefore(text, channelStart)
	if !ok {
		return ParsedDispatch{}, false, "missing_address"
	}
	if previousChannel := lastCDECMarkerStart(text[:addressStart]); previousChannel >= 0 {
		return ParsedDispatch{}, false, "missing_address"
	}
	units := extractUnits(text[addressEnd:channelEnd])
	if len(units) == 0 {
		return ParsedDispatch{}, false, "missing_unit"
	}
	nature := matchKnownNature(text[:addressStart])
	if nature == "" {
		return ParsedDispatch{}, false, "missing_nature"
	}
	channel := extractCDECChannel(text[channelStart:channelEnd])
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
	out := []ExpectedUnit{}
	seen := map[string]bool{}
	for _, match := range findUnitMatches(text) {
		unitName := match.unit.UnitName
		key := strings.ToLower(unitName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, match.unit)
	}
	return out
}

func findAddress(text string) (string, int, bool) {
	address, start, _, ok := findAddressInRange(text, 0, len(text))
	return address, start, ok
}

func findAddressAfter(text string, start int) (string, int, int, bool) {
	return findAddressInRange(text, start, len(text))
}

func findAddressBefore(text string, end int) (string, int, int, bool) {
	return findAddressInRange(text, 0, end)
}

func findAddressInRange(text string, start, end int) (string, int, int, bool) {
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	if start >= end {
		return "", 0, 0, false
	}
	window := text[start:end]
	bestStart := -1
	bestEnd := -1
	for _, pattern := range addressPatterns {
		loc := pattern.FindStringIndex(window)
		if loc == nil {
			continue
		}
		if bestStart == -1 || loc[0] < bestStart {
			bestStart = loc[0]
			bestEnd = loc[1]
		}
	}
	if bestStart == -1 {
		return "", 0, 0, false
	}
	absoluteStart := start + bestStart
	absoluteEnd := start + bestEnd
	address, startOffset := cleanAddress(text[absoluteStart:absoluteEnd])
	if address == "" {
		return "", 0, 0, false
	}
	return address, absoluteStart + startOffset, absoluteEnd, true
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

type unitMatch struct {
	start int
	end   int
	unit  ExpectedUnit
}

func findUnitMatches(text string) []unitMatch {
	matches := []unitMatch{}
	for _, match := range unitPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) < 6 || match[2] < 0 || match[4] < 0 {
			continue
		}
		unitType := canonicalUnitType(text[match[2]:match[3]])
		digits := compactDigits(text[match[4]:match[5]])
		if digits == "" {
			continue
		}
		matches = append(matches, unitMatch{
			start: match[0],
			end:   match[1],
			unit:  ExpectedUnit{UnitName: unitType + " " + digits, UnitType: unitType},
		})
	}
	for _, match := range hyphenatedAMRPattern.FindAllStringSubmatchIndex(text, -1) {
		if len(match) < 4 || match[2] < 0 {
			continue
		}
		digits := compactDigits(text[match[2]:match[3]])
		if digits == "" {
			continue
		}
		matches = append(matches, unitMatch{
			start: match[0],
			end:   match[1],
			unit:  ExpectedUnit{UnitName: "AMR " + digits, UnitType: "AMR"},
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].start == matches[j].start {
			return matches[i].end < matches[j].end
		}
		return matches[i].start < matches[j].start
	})
	return dedupeUnitMatches(matches)
}

func dedupeUnitMatches(matches []unitMatch) []unitMatch {
	out := make([]unitMatch, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		key := strings.ToLower(match.unit.UnitName)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, match)
	}
	return out
}

func compactDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func channelSequenceEnd(text string, start int) int {
	end := start
	for end < len(text) {
		trimmed := strings.TrimLeft(text[end:], " \t\r\n.,;:-")
		offset := len(text[end:]) - len(trimmed)
		loc := cdecPattern.FindStringIndex(trimmed)
		if loc == nil || loc[0] != 0 {
			return end
		}
		end += offset + loc[1]
	}
	return end
}

func nextCDECMarkerStart(text string, start int) int {
	if start >= len(text) {
		return -1
	}
	loc := cdecPattern.FindStringIndex(text[start:])
	if loc == nil {
		return -1
	}
	return start + loc[0]
}

func lastCDECMarkerStart(text string) int {
	matches := cdecPattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return -1
	}
	return matches[len(matches)-1][0]
}

func dispatchSegmentStart(text string, channelStart int) int {
	matches := findUnitMatches(text[:channelStart])
	if len(matches) == 0 {
		return channelStart
	}
	groupStart := matches[len(matches)-1].start
	for i := len(matches) - 2; i >= 0; i-- {
		between := text[matches[i].end:groupStart]
		if strings.ContainsAny(between, ".;") || len(strings.Fields(between)) > 4 {
			break
		}
		groupStart = matches[i].start
	}
	return groupStart
}

func extendSegmentToTrailingUnit(text string, segmentEnd int) int {
	limit := segmentEnd + 100
	if limit > len(text) {
		limit = len(text)
	}
	if nextChannel := nextCDECMarkerStart(text, segmentEnd); nextChannel >= 0 && nextChannel < limit {
		limit = nextChannel
	}
	matches := findUnitMatches(text[segmentEnd:limit])
	if len(matches) == 0 {
		return segmentEnd
	}
	return segmentEnd + matches[0].end
}

func cleanAddress(raw string) (string, int) {
	cleaned := strings.Trim(strings.TrimSpace(raw), ".,")
	startOffset := leadingAddressNoiseOffset(cleaned)
	if startOffset > 0 {
		cleaned = strings.TrimSpace(cleaned[startOffset:])
	}
	cleaned = esuiteDirectionPattern.ReplaceAllString(cleaned, "East")
	cleaned = punctuatedDirectionPattern.ReplaceAllString(cleaned, "$1 $2")
	cleaned = splitHouseNumberPattern.ReplaceAllString(cleaned, "$1$2 $3")
	cleaned = overUnderSuffixPattern.ReplaceAllString(cleaned, "")
	cleaned = whitespacePattern.ReplaceAllString(cleaned, " ")
	cleaned = strings.Trim(strings.TrimSpace(cleaned), ".,")
	return strings.TrimSpace(cleaned), startOffset
}

func leadingAddressNoiseOffset(s string) int {
	lower := strings.ToLower(s)
	best := 0
	for _, nature := range knownDispatchNatures {
		phrases := append([]string{nature.canonical}, nature.phrases...)
		for _, phrase := range phrases {
			prefix := strings.ToLower(phrase) + " "
			if strings.HasPrefix(lower, prefix) && len(prefix) > best {
				best = len(prefix)
			}
		}
	}
	if best > 0 {
		return best
	}
	for _, pattern := range leadingNatureAddressNoisePatterns {
		loc := pattern.FindStringIndex(s)
		if loc != nil && loc[0] == 0 {
			return loc[1]
		}
	}
	return 0
}
