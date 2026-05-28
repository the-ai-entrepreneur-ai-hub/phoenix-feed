package dispatchparser

import "strings"

var knownDispatchNatures = []string{
	"Residential Fire Alarm",
	"Commercial Fire Alarm",
	"Level of Consciousness",
	"Difficulty Breathing",
	"Smoke Investigation",
	"Medical Assignment",
	"Breathing Problems",
	"Cardiac Problems",
	"Cardiac Problem",
	"Unknown Trouble",
	"Vehicle Accident",
	"Vehicle Crash",
	"Structure Fire",
	"Working Fire",
	"Brush Fire",
	"Trash Fire",
	"House Fire",
	"Gas Leak",
	"Sick Person",
	"Welfare Check",
	"Check Welfare",
	"Overdose",
	"Seizure",
	"Assault",
	"Hazmat",
	"Fall",
}

func matchKnownNature(candidate string) string {
	normalizedCandidate := " " + normalizeNatureForMatch(candidate) + " "
	if strings.TrimSpace(normalizedCandidate) == "" {
		return ""
	}

	best := ""
	bestIndex := -1
	for _, nature := range knownDispatchNatures {
		needle := " " + normalizeNatureForMatch(nature) + " "
		index := strings.Index(normalizedCandidate, needle)
		if index < 0 {
			continue
		}
		if bestIndex == -1 || index < bestIndex || (index == bestIndex && len(nature) > len(best)) {
			best = nature
			bestIndex = index
		}
	}
	return best
}

func normalizeNatureForMatch(s string) string {
	var b strings.Builder
	lastSpace := true
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}
