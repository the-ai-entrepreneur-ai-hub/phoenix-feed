package dispatchparser

import "strings"

type dispatchNaturePhrase struct {
	canonical string
	phrases   []string
}

var knownDispatchNatures = []dispatchNaturePhrase{
	{canonical: "Motor Vehicle Accident With Motorcycle"},
	{canonical: "Residential Fire Alarm"},
	{canonical: "Commercial Fire Alarm"},
	{canonical: "Level of Consciousness"},
	{canonical: "Difficulty Breathing"},
	{canonical: "Smoke Investigation"},
	{canonical: "Medical Assignment"},
	{canonical: "Breathing Problems"},
	{canonical: "Cardiac Problems"},
	{canonical: "Cardiac Problem"},
	{canonical: "Unknown Trouble"},
	{canonical: "Motor Vehicle Accident"},
	{canonical: "Vehicle Accident"},
	{canonical: "Vehicle Crash"},
	{canonical: "Structure Fire"},
	{canonical: "Working Fire"},
	{canonical: "Brush Fire"},
	{canonical: "Trash Fire"},
	{canonical: "House Fire"},
	{canonical: "Injured Person"},
	{canonical: "Animal Issue"},
	{canonical: "Ill Person", phrases: []string{"Hill Person"}},
	{canonical: "Gas Leak", phrases: []string{"Natural Gas Leak"}},
	{canonical: "Sick Person"},
	{canonical: "Welfare Check"},
	{canonical: "Check Welfare"},
	{canonical: "Overdose"},
	{canonical: "Seizure"},
	{canonical: "Assault"},
	{canonical: "Hazmat"},
	{canonical: "Stroke"},
	{canonical: "Fall"},
}

func matchKnownNature(candidate string) string {
	normalizedCandidate := " " + normalizeNatureForMatch(candidate) + " "
	if strings.TrimSpace(normalizedCandidate) == "" {
		return ""
	}

	best := ""
	bestIndex := -1
	bestLength := 0
	for _, nature := range knownDispatchNatures {
		phrases := append([]string{nature.canonical}, nature.phrases...)
		for _, phrase := range phrases {
			needle := " " + normalizeNatureForMatch(phrase) + " "
			index := strings.Index(normalizedCandidate, needle)
			if index < 0 {
				continue
			}
			if bestIndex == -1 || index < bestIndex || (index == bestIndex && len(phrase) > bestLength) {
				best = nature.canonical
				bestIndex = index
				bestLength = len(phrase)
			}
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
