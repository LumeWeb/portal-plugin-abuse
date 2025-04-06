package util

import (
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/queryutil"
	"time"
)

// ApplyTimeRangeFilters extracts and validates time range from filters.
// Returns cleaned filters without the time range entries and any error.
func ApplyTimeRangeFilters(filters []queryutil.CrudFilter, timeField string) ([]queryutil.CrudFilter, error) {
	var startDate, endDate time.Time
	var err error

	// Find time range filters
	gteFilter := queryutil.FindFilterWithOperator(filters, timeField, queryutil.OpGte)
	if gteFilter != nil {
		startDate, err = parseTimeFromFilter(gteFilter)
		if err != nil {
			return filters, fmt.Errorf("invalid start time in filter: %w", err)
		}
	}

	lteFilter := queryutil.FindFilterWithOperator(filters, timeField, queryutil.OpLte)
	if lteFilter != nil {
		endDate, err = parseTimeFromFilter(lteFilter)
		if err != nil {
			return filters, fmt.Errorf("invalid end time in filter: %w", err)
		}
	}

	// Validate time range if both dates present
	if !startDate.IsZero() && !endDate.IsZero() && startDate.After(endDate) {
		return filters, fmt.Errorf("start date cannot be after end date")
	}

	// Remove time range filters from the filters slice
	newFilters := lo.Filter(filters, func(f queryutil.CrudFilter, _ int) bool {
		return !(f.GetField() == timeField && (f.GetOperator() == queryutil.OpGte || f.GetOperator() == queryutil.OpLte))
	})

	return newFilters, nil
}

func parseTimeFromFilter(filter queryutil.CrudFilter) (time.Time, error) {
	var t time.Time
	var err error

	switch v := filter.GetValue().(type) {
	case time.Time:
		t = v
	case string:
		t, err = ParseTime(v)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse time string: %w", err)
		}
	default:
		return time.Time{}, fmt.Errorf("unsupported time filter value type: %T", filter.GetValue())
	}

	return t, nil
}
