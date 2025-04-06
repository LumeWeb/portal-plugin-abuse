package util

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidTimeRange = errors.New("invalid timeRange")
	timeFormats         = []string{
		time.RFC3339,
		"2006-01-02 15:04:05.999999999+00:00", // Common MySQL format with nanoseconds
		"2006-01-02 15:04:05+00:00",           // MySQL format without nanoseconds
		"2006-01-02 15:04:05.999999999",       // Without timezone
		"2006-01-02 15:04:05",                 // Basic datetime format
		"2006-01-02",                          // Basic date format
	}
)

// ParseTime attempts to parse a time string using multiple common formats
func ParseTime(timeStr string) (time.Time, error) {
	for _, format := range timeFormats {
		t, err := time.Parse(format, timeStr)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse time '%s' with any known format", timeStr)
}

func ParseTimeRange(timeRange string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	var duration time.Duration
	switch timeRange {
	case "24h":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	case "90d":
		duration = 90 * 24 * time.Hour
	default:
		return time.Time{}, time.Time{}, ErrInvalidTimeRange
	}

	startDate := now.Add(-duration)
	return startDate, now, nil
}
