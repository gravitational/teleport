// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package discovery

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// compareTimeDesc compares two times in descending order (newest first).
// Zero times are considered equal to each other and older than any non-zero time.
func compareTimeDesc(a, b time.Time) int {
	switch {
	case a.IsZero() && b.IsZero():
		return 0
	case a.IsZero():
		return 1
	case b.IsZero():
		return -1
	case a.After(b):
		return -1
	case b.After(a):
		return 1
	default:
		return 0
	}
}

func sortSSMRunRecords(records []ssmRunRecord) {
	slices.SortFunc(records, func(a, b ssmRunRecord) int {
		if c := compareTimeDesc(a.parsedEventTime, b.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(b.EventTime, a.EventTime)
	})
}

func sortJoinRecords(records []joinRecord) {
	slices.SortFunc(records, func(a, b joinRecord) int {
		if c := compareTimeDesc(a.parsedEventTime, b.parsedEventTime); c != 0 {
			return c
		}
		return cmp.Compare(b.EventTime, a.EventTime)
	})
}

func parseAuditEventTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	withTZLayouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range withTZLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), true
		}
	}

	withoutTZLayouts := []string{
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05.999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range withoutTZLayouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}

	return time.Time{}, false
}

// groupByAccountField groups items by account ID extracted via getAccount.
// Items with empty/whitespace account IDs are grouped under "unknown".
func groupByAccountField[T any](items []T, getAccount func(T) string) map[string][]T {
	grouped := map[string][]T{}
	for _, item := range items {
		acct := cmp.Or(strings.TrimSpace(getAccount(item)), "unknown")
		grouped[acct] = append(grouped[acct], item)
	}
	return grouped
}

type countRow struct {
	Key   string
	Count int
}

func countRows(counts map[string]int) []countRow {
	keys := mapKeys(counts)
	rows := make([]countRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, countRow{Key: key, Count: counts[key]})
	}
	slices.SortFunc(rows, func(a, b countRow) int {
		if a.Count != b.Count {
			if a.Count > b.Count {
				return -1
			}
			return 1
		}
		return cmp.Compare(a.Key, b.Key)
	})
	return rows
}
