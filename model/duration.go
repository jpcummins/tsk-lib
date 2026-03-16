package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Duration represents a parsed duration token (e.g., "2h", "1.5d", "1w").
// Stored internally as minutes for uniform comparison and arithmetic.
type Duration struct {
	Minutes int
	Raw     string // Original token string.
}

// Unit multipliers in minutes.
const (
	minutesPerMinute = 1
	minutesPerHour   = 60
	minutesPerDay    = 480  // 8-hour workday
	minutesPerWeek   = 2400 // 5 * 8-hour workday
)

// ParseDuration parses a duration token like "2h", "1.5d", "30m", "1w".
// Supports units: m (minutes), h (hours), d (days/8h), w (weeks/5*8h).
// Negative values are allowed (e.g., "-2h").
func ParseDuration(s string) (Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return Duration{}, fmt.Errorf("empty duration string")
	}

	unit := s[len(s)-1:]
	numStr := s[:len(s)-1]

	var multiplier int
	switch unit {
	case "m":
		multiplier = minutesPerMinute
	case "h":
		multiplier = minutesPerHour
	case "d":
		multiplier = minutesPerDay
	case "w":
		multiplier = minutesPerWeek
	default:
		return Duration{}, fmt.Errorf("unknown duration unit %q in %q", unit, s)
	}

	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return Duration{}, fmt.Errorf("invalid duration number %q in %q: %w", numStr, s, err)
	}

	minutes := int(val * float64(multiplier))
	return Duration{Minutes: minutes, Raw: s}, nil
}

// TimeDuration converts to a standard time.Duration (using real minutes).
func (d Duration) TimeDuration() time.Duration {
	return time.Duration(d.Minutes) * time.Minute
}

// String returns the original raw token if available, otherwise a computed representation.
func (d Duration) String() string {
	if d.Raw != "" {
		return d.Raw
	}
	return fmt.Sprintf("%dm", d.Minutes)
}

// IsZero returns true if the duration is zero.
func (d Duration) IsZero() bool {
	return d.Minutes == 0
}
