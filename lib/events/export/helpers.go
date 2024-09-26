package export

import "time"

// normalizeDate normalizes a timestamp to the beginning of the day in UTC.
func normalizeDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
