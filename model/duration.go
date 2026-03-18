package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration represents a work-time duration stored as minutes.
// Work time assumptions: 8 hours/day, 5 days/week.
type Duration int64

const (
	MinutesPerHour = 60
	HoursPerDay    = 8
	DaysPerWeek    = 5
	DaysPerMonth   = 20 // 4 weeks

	MinutesPerDay   = MinutesPerHour * HoursPerDay // 480
	MinutesPerWeek  = MinutesPerDay * DaysPerWeek  // 2400
	MinutesPerMonth = MinutesPerDay * DaysPerMonth // 9600
)

// ParseDuration parses a duration string like "2h", "1.5d", "1w", "30m".
// Units: h (hours), d (days), w (weeks), m (months).
// Decimals are allowed. No spaces between number and unit.
func ParseDuration(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration string")
	}

	// Find the unit suffix
	unit := s[len(s)-1:]
	numStr := s[:len(s)-1]

	if numStr == "" {
		return 0, fmt.Errorf("invalid duration: %q", s)
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number %q: %w", numStr, err)
	}

	var minutes float64
	switch unit {
	case "h":
		minutes = val * float64(MinutesPerHour)
	case "d":
		minutes = val * float64(MinutesPerDay)
	case "w":
		minutes = val * float64(MinutesPerWeek)
	case "m":
		minutes = val * float64(MinutesPerMonth)
	default:
		return 0, fmt.Errorf("unknown duration unit %q in %q", unit, s)
	}

	return Duration(minutes), nil
}

// Minutes returns the total minutes.
func (d Duration) Minutes() int64 {
	return int64(d)
}

// Hours returns the duration in hours.
func (d Duration) Hours() float64 {
	return float64(d) / float64(MinutesPerHour)
}

// Days returns the duration in work days.
func (d Duration) Days() float64 {
	return float64(d) / float64(MinutesPerDay)
}

// String returns a human-readable representation.
func (d Duration) String() string {
	mins := int64(d)
	if mins == 0 {
		return "0h"
	}

	negative := mins < 0
	if negative {
		mins = -mins
	}

	prefix := ""
	if negative {
		prefix = "-"
	}

	if mins >= int64(MinutesPerMonth) && mins%int64(MinutesPerMonth) == 0 {
		return fmt.Sprintf("%s%dm", prefix, mins/int64(MinutesPerMonth))
	}
	if mins >= int64(MinutesPerWeek) && mins%int64(MinutesPerWeek) == 0 {
		return fmt.Sprintf("%s%dw", prefix, mins/int64(MinutesPerWeek))
	}
	if mins >= int64(MinutesPerDay) && mins%int64(MinutesPerDay) == 0 {
		return fmt.Sprintf("%s%dd", prefix, mins/int64(MinutesPerDay))
	}
	if mins%int64(MinutesPerHour) == 0 {
		return fmt.Sprintf("%s%dh", prefix, mins/int64(MinutesPerHour))
	}

	// Fall back to fractional hours
	hours := float64(mins) / float64(MinutesPerHour)
	return fmt.Sprintf("%s%.1fh", prefix, hours)
}

// ToDuration converts a work-time Duration to a standard time.Duration
// using wall-clock equivalents (for SLA elapsed time comparisons).
func (d Duration) ToDuration() time.Duration {
	return time.Duration(int64(d)) * time.Minute
}
