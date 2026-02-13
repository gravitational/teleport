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
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type pageInfo struct {
	Page      int
	PageSize  int
	Total     int
	Start     int
	End       int
	Remaining int
	HasNext   bool
	NextPage  int
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 25
	}
	return page, pageSize
}

func paginateSlice[T any](items []T, page, pageSize int) ([]T, pageInfo) {
	page, pageSize = normalizePage(page, pageSize)
	total := len(items)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	info := pageInfo{
		Page:      page,
		PageSize:  pageSize,
		Total:     total,
		Start:     start,
		End:       end,
		Remaining: total - end,
		HasNext:   end < total,
		NextPage:  page + 1,
	}
	return items[start:end], info
}

func fullPageInfo(n int) pageInfo {
	return pageInfo{
		Page:     1,
		PageSize: n,
		Total:    n,
		End:      n,
		NextPage: 2,
	}
}

var pageFlagPattern = regexp.MustCompile(`(?:^|\s)--page=\S+`)
var markdownLinkPattern = regexp.MustCompile(`\[(.+?)\]\((https?://[^)]+)\)`)
var markdownStrongPattern = regexp.MustCompile(`\*\*(.+?)\*\*`)
var markdownCodePattern = regexp.MustCompile("`([^`]+)`")

func withPageFlag(command string, page int) string {
	command = strings.TrimSpace(command)
	command = pageFlagPattern.ReplaceAllString(command, "")
	command = strings.Join(strings.Fields(command), " ")
	return fmt.Sprintf("%s --page=%d", command, page)
}

func formatTime(ts time.Time) string {
	if ts.IsZero() {
		return "n/a"
	}
	return ts.UTC().Format(time.RFC3339)
}

func formatProtoTimestamp(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return "n/a"
	}
	return formatTime(ts.AsTime())
}

func mapKeys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func paginateMapKeys[V any](w io.Writer, m map[string]V, page, pageSize int) ([]string, pageInfo, textStyle) {
	style := newTextStyle(w)
	keys := mapKeys(m)
	slices.Sort(keys)
	pageKeys, info := paginateSlice(keys, page, pageSize)
	return pageKeys, info, style
}

func humanizeEnumValue(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "Unknown"
	}
	value = strings.TrimPrefix(value, "DISCOVERY_CONFIG_STATE_")
	value = strings.Trim(value, "_")
	if value == "" {
		return "Unknown"
	}
	value = strings.ToLower(value)
	words := strings.Split(value, "_")
	for i := range words {
		if words[i] == "" {
			continue
		}
		words[i] = strings.ToUpper(words[i][:1]) + words[i][1:]
	}
	return strings.Join(words, " ")
}

func formatRelativeTime(ts time.Time, now time.Time) string {
	return formatRelativeDelta(ts, now, false)
}

func formatRelativeDelta(ts time.Time, now time.Time, detailed bool) string {
	if ts.IsZero() {
		return "never"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	delta := now.Sub(ts)
	suffix := "ago"
	if delta < 0 {
		delta = -delta
		suffix = "from now"
	}

	totalMinutes := int64(delta / time.Minute)
	if totalMinutes < 1 {
		if !detailed {
			secs := int64(delta / time.Second)
			if secs < 1 {
				secs = 1
			}
			return strconv.FormatInt(secs, 10) + "s " + suffix
		}
		totalMinutes = 1
	}

	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	if !detailed {
		switch {
		case days > 0:
			return strconv.FormatInt(days, 10) + "d " + suffix
		case hours > 0:
			return strconv.FormatInt(hours, 10) + "h " + suffix
		default:
			return strconv.FormatInt(minutes, 10) + "m " + suffix
		}
	}

	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd %dh %s", days, hours, suffix)
		}
		return fmt.Sprintf("%dd %s", days, suffix)
	case hours > 0:
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm %s", hours, minutes, suffix)
		}
		return fmt.Sprintf("%dh %s", hours, suffix)
	default:
		return fmt.Sprintf("%dm %s", minutes, suffix)
	}
}

func formatHelpText(description string) string {
	text := strings.TrimSpace(description)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	formatted := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			formatted = append(formatted, "")
			continue
		}
		trimmed = markdownLinkPattern.ReplaceAllString(trimmed, "$1: $2")
		trimmed = markdownCodePattern.ReplaceAllString(trimmed, "$1")

		if match := markdownStrongPattern.FindStringSubmatch(trimmed); len(match) == 2 && trimmed == match[0] {
			trimmed = strings.ToUpper(strings.TrimSpace(match[1])) + ":"
		} else {
			trimmed = markdownStrongPattern.ReplaceAllString(trimmed, "$1")
		}
		formatted = append(formatted, trimmed)
	}
	return strings.Join(formatted, "\n")
}

func formatExpiryTime(ts time.Time, now time.Time) string {
	relative := formatRelativeTime(ts, now)
	if relative == "never" {
		return relative
	}
	if strings.HasSuffix(relative, " from now") {
		return "in " + strings.TrimSuffix(relative, " from now")
	}
	return "expired " + relative
}

func formatCountLabel(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func formatRelativeOrTimestamp(parsed time.Time, raw string, now time.Time) string {
	if !parsed.IsZero() {
		return formatRelativeTime(parsed, now)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "never"
	}
	return "timestamp: " + raw
}

func formatMaybeParsedTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format("2006-01-02 15:04:05")
}

func formatHistoryTimestamp(timestamp string, now time.Time) string {
	timestamp = strings.TrimSpace(timestamp)
	if timestamp == "" {
		return "never"
	}
	parsed, ok := parseAuditEventTime(timestamp)
	if !ok {
		return timestamp
	}
	return fmt.Sprintf("%s (%s)", timestamp, formatRelativeDelta(parsed, now, true))
}
