package enricher

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	monthMap = map[string]int{
		"jan": 1, "january": 1, "feb": 2, "february": 2,
		"mar": 3, "march": 3, "apr": 4, "april": 4,
		"may": 5, "jun": 6, "june": 6,
		"jul": 7, "july": 7, "aug": 8, "august": 8,
		"sep": 9, "september": 9, "oct": 10, "october": 10,
		"nov": 11, "november": 11, "dec": 12, "december": 12,
	}

	dateTypePrefixes = map[string]DateType{
		"abt": DateAbout, "abt.": DateAbout, "about": DateAbout,
		"c.": DateAbout, "ca": DateAbout, "ca.": DateAbout,
		"cca": DateAbout, "cca.": DateAbout, "circa": DateAbout,
		"bef": DateBefore, "bef.": DateBefore, "before": DateBefore,
		"aft": DateAfter, "aft.": DateAfter, "after": DateAfter,
		"bet": DateBetween, "bet.": DateBetween, "between": DateBetween,
		"from":  DateFromTo,
		"cal":   DateCalculated,
		"cal.":  DateCalculated,
		"est":   DateEstimated,
		"est.":  DateEstimated,
	}

	exactDateRe  = regexp.MustCompile(`(?i)^(\d{1,2})\s+(\w+)\s+(\d{1,4})$`)
	monthYearRe  = regexp.MustCompile(`(?i)^(\w+)\s+(\d{1,4})$`)
	yearOnlyRe   = regexp.MustCompile(`^\d{1,4}$`)
	betweenRe    = regexp.MustCompile(`(?i)^(?:bet\.?|between)\s+(.+?)\s+(?:and|-)\s+(.+)$`)
	fromToRe     = regexp.MustCompile(`(?i)^from\s+(.+?)\s+to\s+(.+)$`)
	calendarTagRe = regexp.MustCompile(`(?i)^@#D(\w+)@\s*(.*)$`)
)

// ParseDateString converts a raw GEDCOM date string into a ParsedDate.
func ParseDateString(original string) ParsedDate {
	pd := ParsedDate{
		Original: original,
		Type:     DateUnknown,
		Calendar: "GREGORIAN",
	}

	if strings.TrimSpace(original) == "" {
		pd.Hash = hashDateFields(pd)
		return pd
	}

	normalized := strings.TrimSpace(original)

	// Check for calendar escape (e.g., @#DJULIAN@ 5 OCT 1582)
	if m := calendarTagRe.FindStringSubmatch(normalized); m != nil {
		pd.Calendar = strings.ToUpper(m[1])
		normalized = strings.TrimSpace(m[2])
	}

	lower := strings.ToLower(normalized)
	parts := strings.Fields(lower)

	if len(parts) == 0 {
		pd.Hash = hashDateFields(pd)
		return pd
	}

	// Detect date type prefix
	if dt, ok := dateTypePrefixes[parts[0]]; ok {
		switch dt {
		case DateBetween:
			pd.Type = DateBetween
			parseBetweenDate(&pd, lower)
			pd.Hash = hashDateFields(pd)
			return pd
		case DateFromTo:
			// Could be FROM..TO or just FROM with no TO
			if m := fromToRe.FindStringSubmatch(lower); m != nil {
				pd.Type = DateFromTo
				parseRangeHalves(&pd, m[1], m[2])
				pd.Hash = hashDateFields(pd)
				return pd
			}
			// FROM without TO — treat as AFTER
			pd.Type = DateAfter
			rest := strings.Join(parts[1:], " ")
			parseSingleDate(&pd, rest)
			pd.Hash = hashDateFields(pd)
			return pd
		default:
			pd.Type = dt
			lower = strings.Join(parts[1:], " ")
		}
	}

	// Default: parse as single date
	parseSingleDate(&pd, lower)

	if pd.Type == DateUnknown && (pd.Year != 0 || pd.Month != 0 || pd.Day != 0) {
		pd.Type = DateExact
	}

	pd.Hash = hashDateFields(pd)
	return pd
}

func parseSingleDate(pd *ParsedDate, s string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}

	// "15 JAN 1800"
	if m := exactDateRe.FindStringSubmatch(s); m != nil {
		pd.Day, _ = strconv.Atoi(m[1])
		if mon, ok := monthMap[strings.ToLower(m[2])]; ok {
			pd.Month = mon
		}
		pd.Year, _ = strconv.Atoi(m[3])
		return
	}

	// "JAN 1800"
	if m := monthYearRe.FindStringSubmatch(s); m != nil {
		if mon, ok := monthMap[strings.ToLower(m[1])]; ok {
			pd.Month = mon
			pd.Year, _ = strconv.Atoi(m[2])
			return
		}
	}

	// "1800"
	if yearOnlyRe.MatchString(s) {
		pd.Year, _ = strconv.Atoi(s)
		return
	}
}

func parseBetweenDate(pd *ParsedDate, s string) {
	if m := betweenRe.FindStringSubmatch(s); m != nil {
		parseRangeHalves(pd, m[1], m[2])
	}
}

func parseRangeHalves(pd *ParsedDate, startStr, endStr string) {
	var startPd, endPd ParsedDate
	parseSingleDate(&startPd, strings.TrimSpace(startStr))
	parseSingleDate(&endPd, strings.TrimSpace(endStr))

	pd.Year = startPd.Year
	pd.Month = startPd.Month
	pd.Day = startPd.Day
	pd.EndYear = endPd.Year
	pd.EndMonth = endPd.Month
	pd.EndDay = endPd.Day
}

func hashDateFields(pd ParsedDate) string {
	raw := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d|%d|%d",
		pd.Original, pd.Type, pd.Calendar,
		pd.Year, pd.Month, pd.Day,
		pd.EndYear, pd.EndMonth, pd.EndDay)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum[:16])
}

var gedcomMonthAbbrev = [...]string{
	0:  "",
	1:  "JAN",
	2:  "FEB",
	3:  "MAR",
	4:  "APR",
	5:  "MAY",
	6:  "JUN",
	7:  "JUL",
	8:  "AUG",
	9:  "SEP",
	10: "OCT",
	11: "NOV",
	12: "DEC",
}

// formatSingleGEDCOMDatePoint renders day/month/year using GEDCOM month tokens.
// Year 0 means “no date”; month 0 with year yields year-only; day 0 with month yields MON YYYY.
func formatSingleGEDCOMDatePoint(day, month, year int) string {
	if year == 0 {
		return ""
	}
	if month >= 1 && month <= 12 && day > 0 {
		return fmt.Sprintf("%d %s %d", day, gedcomMonthAbbrev[month], year)
	}
	if month >= 1 && month <= 12 {
		return fmt.Sprintf("%s %d", gedcomMonthAbbrev[month], year)
	}
	return strconv.Itoa(year)
}

// FormatGEDCOMDate renders a ParsedDate as a GEDCOM 5.5–style DATE payload when
// structured fields are available; otherwise returns trimmed Original. Returns ""
// when there is no usable date text (omit a DATE line in that case).
func FormatGEDCOMDate(pd ParsedDate) string {
	orig := strings.TrimSpace(pd.Original)

	var inner string
	switch pd.Type {
	case DateBetween:
		a := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year)
		b := formatSingleGEDCOMDatePoint(pd.EndDay, pd.EndMonth, pd.EndYear)
		if a != "" && b != "" {
			inner = "BET " + a + " AND " + b
		}
	case DateFromTo:
		a := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year)
		b := formatSingleGEDCOMDatePoint(pd.EndDay, pd.EndMonth, pd.EndYear)
		if a != "" && b != "" {
			inner = "FROM " + a + " TO " + b
		}
	case DateAbout:
		if p := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year); p != "" {
			inner = "ABT " + p
		}
	case DateBefore:
		if p := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year); p != "" {
			inner = "BEF " + p
		}
	case DateAfter:
		if p := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year); p != "" {
			inner = "AFT " + p
		}
	case DateCalculated:
		if p := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year); p != "" {
			inner = "CAL " + p
		}
	case DateEstimated:
		if p := formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year); p != "" {
			inner = "EST " + p
		}
	default:
		inner = formatSingleGEDCOMDatePoint(pd.Day, pd.Month, pd.Year)
	}

	if inner == "" {
		return orig
	}
	if pd.Calendar != "" && !strings.EqualFold(pd.Calendar, "GREGORIAN") {
		return "@#D" + strings.ToUpper(strings.TrimSpace(pd.Calendar)) + "@ " + inner
	}
	return inner
}
