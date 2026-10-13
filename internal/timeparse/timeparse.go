package timeparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Parse reads a time as Unix seconds, a date, an RFC 3339 timestamp or a
// duration back from now, such as 30d or 1h.
func Parse(input string) (int64, error) {
	return ParseAt(input, time.Now())
}

// ParseAt is Parse with relative durations measured back from now. A command
// that parses several times and plans requests from them should use one now,
// so that a range such as --since 31d is exactly 31 days long.
func ParseAt(input string, now time.Time) (int64, error) {
	if isAllDigits(input) {
		return strconv.ParseInt(input, 10, 64)
	}

	if dur, ok := parseRelativeDuration(input); ok {
		return now.Add(-dur).Unix(), nil
	}

	if t, err := time.Parse("2006-01-02", input); err == nil {
		return t.Unix(), nil
	}

	if t, err := time.Parse(time.RFC3339, input); err == nil {
		return t.Unix(), nil
	}

	return 0, fmt.Errorf("unrecognized time format: %q", input)
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

var durationUnits = map[string]time.Duration{
	"d": 24 * time.Hour,
	"h": time.Hour,
	"m": time.Minute,
	"s": time.Second,
}

func parseRelativeDuration(input string) (time.Duration, bool) {
	input = strings.TrimSpace(input)
	if len(input) < 2 {
		return 0, false
	}

	unit := input[len(input)-1:]
	multiplier, ok := durationUnits[unit]
	if !ok {
		return 0, false
	}

	n, err := strconv.ParseInt(input[:len(input)-1], 10, 64)
	if err != nil {
		return 0, false
	}

	return time.Duration(n) * multiplier, true
}
