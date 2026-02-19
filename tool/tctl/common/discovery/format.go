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

const (
	// placeholderNA is used when a value is not available.
	placeholderNA = "n/a"
	// placeholderNone is used when a value is explicitly empty/absent.
	placeholderNone = "-"
)

type pageInfo struct {
	Start     int `json:"start"`
	End       int `json:"end"`
	Total     int `json:"total"`
	Remaining int `json:"remaining"`
	HasNext   bool `json:"has_next"`
}

func parseRange(input string) (int, int, error) {
	parts := strings.SplitN(input, ",", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q, expected format start,end (e.g. 0,25)", input)
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range start %q: %w", parts[0], err)
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid range end %q: %w", parts[1], err)
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("range start must be non-negative, got %d", start)
	}
	if end < start {
		return 0, 0, fmt.Errorf("range end (%d) must be >= start (%d)", end, start)
	}
	return start, end, nil
}

func paginateSlice[T any](items []T, start, end int) ([]T, pageInfo) {
	total := len(items)
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	info := pageInfo{
		Start:     start,
		End:       end,
		Total:     total,
		Remaining: total - end,
		HasNext:   end < total,
	}
	return items[start:end], info
}

func fullPageInfo(n int) pageInfo {
	return pageInfo{
		Start: 0,
		End:   n,
		Total: n,
	}
}

var rangeFlagPattern = regexp.MustCompile(`(?:^|\s)--range=\S+`)
var markdownLinkPattern = regexp.MustCompile(`\[(.+?)\]\((https?://[^)]+)\)`)
var markdownStrongPattern = regexp.MustCompile(`\*\*(.+?)\*\*`)
var markdownCodePattern = regexp.MustCompile("`([^`]+)`")

func withRangeFlag(command string, start, end int) string {
	command = strings.TrimSpace(command)
	command = rangeFlagPattern.ReplaceAllString(command, "")
	command = strings.Join(strings.Fields(command), " ")
	return fmt.Sprintf("%s --range=%d,%d", command, start, end)
}

func shellQuoteArg(s string) string {
	if s == "" || strings.ContainsAny(s, " \t()\"'\\$`!") {
		return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
	}
	return s
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

func paginateMapKeys[V any](w io.Writer, m map[string]V, start, end int) ([]string, pageInfo, textStyle) {
	style := newTextStyle(w)
	keys := mapKeys(m)
	slices.Sort(keys)
	pageKeys, info := paginateSlice(keys, start, end)
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

// formatDurationParts formats a duration as a human-readable string like "3d", "2h", "5m".
// When detailed is true, includes sub-units (e.g., "2h 30m", "1d 5h").
func formatDurationParts(d time.Duration, detailed bool) string {
	if d < 0 {
		d = -d
	}

	totalMinutes := int64(d / time.Minute)
	if totalMinutes < 1 {
		if !detailed {
			secs := int64(d / time.Second)
			if secs < 1 {
				secs = 1
			}
			return strconv.FormatInt(secs, 10) + "s"
		}
		totalMinutes = 1
	}

	days := totalMinutes / (24 * 60)
	hours := (totalMinutes % (24 * 60)) / 60
	minutes := totalMinutes % 60

	if !detailed {
		switch {
		case days > 0:
			return strconv.FormatInt(days, 10) + "d"
		case hours > 0:
			return strconv.FormatInt(hours, 10) + "h"
		default:
			return strconv.FormatInt(minutes, 10) + "m"
		}
	}

	switch {
	case days > 0:
		if hours > 0 {
			return fmt.Sprintf("%dd %dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	case hours > 0:
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
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
		suffix = "from now"
	}
	return formatDurationParts(delta, detailed) + " " + suffix
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
	if trimmed, ok := strings.CutSuffix(relative, " from now"); ok {
		return "in " + trimmed
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

func combineOutput(stdout, stderr string) string {
	stdout = strings.TrimSpace(stdout)
	stderr = strings.TrimSpace(stderr)
	if stdout == "" && stderr == "" {
		return ""
	}
	if stdout == "" {
		return stderr
	}
	if stderr == "" {
		return stdout
	}
	return stdout + "\n" + stderr
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
