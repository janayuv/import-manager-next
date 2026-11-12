package validation

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var acceptedDateLayouts = []string{
	"2006-01-02",
	"02-01-2006",
	"02/01/2006",
	"2006/01/02",
	"02-Jan-2006",
	"2-Jan-2006",
	"02 Jan 2006",
	"2006-01-02T15:04:05Z",
	"2006-01-02 15:04:05",
}

// NormalizeDate parses ISO, DMY, Excel serial day numbers (e.g. 45124), or RFC3339 timestamps into YYYY-MM-DD.
func NormalizeDate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}

	// Check numeric Excel serial date (between 1950 and 2100 roughly: 18264..73415).
	if num, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(num) && num >= 18000 && num <= 80000 {
		days := int(math.Floor(num))
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return excelEpoch.AddDate(0, 0, days).Format("2006-01-02"), nil
	}

	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}

	for _, layout := range acceptedDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("unrecognized date format %q", raw)
}
