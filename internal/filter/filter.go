package filter

import (
	"regexp"
	"strings"
)

type FilterMatch struct {
	ColIndex int
	Regex    *regexp.Regexp
}

func ParseFilters(filterStrs []string, headers []string) ([]FilterMatch, []string) {
	var filters []FilterMatch
	var rawFilters []string

	for _, fStr := range filterStrs {
		colIdx := -1
		patternStr := fStr

		if strings.Contains(fStr, ":") {
			parts := strings.SplitN(fStr, ":", 2)
			potentialCol := strings.TrimSpace(parts[0])
			if !strings.Contains(potentialCol, " ") && len(potentialCol) <= 15 {
				colSpecLower := strings.ToLower(potentialCol)
				foundIdx := -1
				for idx, h := range headers {
					cleanH := strings.ToLower(h)
					cleanH = strings.NewReplacer(" ", "", "/", "", "$", "", "(", "", ")", "").Replace(cleanH)
					cleanCol := strings.NewReplacer(" ", "", "-", "").Replace(colSpecLower)
					if strings.Contains(cleanH, cleanCol) || strings.HasPrefix(cleanH, cleanCol) {
						foundIdx = idx
						break
					}
				}
				if foundIdx != -1 {
					colIdx = foundIdx
					patternStr = parts[1]
				}
			}
		}

		re, err := regexp.Compile("(?i)" + strings.TrimSpace(patternStr))
		if err == nil {
			filters = append(filters, FilterMatch{ColIndex: colIdx, Regex: re})
		} else {
			rawFilters = append(rawFilters, fStr)
		}
	}

	return filters, rawFilters
}

func EvaluateRow(row []string, filters []FilterMatch) bool {
	for _, f := range filters {
		if f.ColIndex >= 0 && f.ColIndex < len(row) {
			if !f.Regex.MatchString(row[f.ColIndex]) {
				return false
			}
		} else {
			matchedAny := false
			for _, val := range row {
				if f.Regex.MatchString(val) {
					matchedAny = true
					break
				}
			}
			if !matchedAny {
				return false
			}
		}
	}
	return true
}
