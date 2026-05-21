/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package accessgraph

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
)

type timeValue struct {
	target *time.Time
}

func (v timeValue) Set(s string) error {
	parsed, err := parseTimeFilterValue(s, time.Now())
	if err != nil {
		return trace.Wrap(err)
	}

	*v.target = parsed
	return nil
}

func (v timeValue) String() string {
	if v.target == nil || v.target.IsZero() {
		return ""
	}

	return v.target.Format(time.RFC3339)
}

// optionalFloat32 is a kingpin Value that distinguishes 0 from "not set".
type optionalFloat32 struct {
	target **float32
}

func (v optionalFloat32) Set(s string) error {
	f, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return trace.BadParameter("invalid float %q: %v", s, err)
	}
	f32 := float32(f)
	*v.target = &f32
	return nil
}

func (v optionalFloat32) String() string {
	if v.target == nil || *v.target == nil {
		return ""
	}
	return strconv.FormatFloat(float64(**v.target), 'f', -1, 32)
}

func parseTimeFilterValue(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	if s == "now" {
		return now, nil
	}

	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts, nil
	}

	// Date-only inputs are interpreted as local-midnight so a user writing
	// `--from 2026-05-06` gets the start of that day in their own timezone.
	if ts, err := time.ParseInLocation(time.DateOnly, s, time.Local); err == nil {
		return ts, nil
	}

	duration, err := parseRelativeDuration(s)
	if err != nil {
		return time.Time{}, trace.BadParameter("invalid time %q, expected RFC3339 (2026-05-06T15:04:05Z), date (2026-05-06), or relative duration like 24h or 7d", s)
	}

	return now.Add(-duration), nil
}

// validateTimeWindow rejects inverted `--from`/`--to` windows; zero values on
// either side are skipped so callers with optional bounds still work.
func validateTimeWindow(from, to time.Time) error {
	if from.IsZero() || to.IsZero() {
		return nil
	}
	if !from.Before(to) {
		return trace.BadParameter("invalid time window: --from (%s) must be before --to (%s)", from.Format(time.RFC3339), to.Format(time.RFC3339))
	}
	return nil
}

// parseRelativeDuration extends time.ParseDuration with a "d" suffix meaning days.
func parseRelativeDuration(s string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(s, "d"); ok {
		hours, err := time.ParseDuration(before + "h")
		if err != nil {
			return 0, trace.Wrap(err)
		}
		return hours * 24, nil
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return duration, nil
}

// fetchAllLogs paginates ExecuteLogsQueryV1 up to maxResults events
// (<=0 means unbounded); truncated is true if more results remained.
func fetchAllLogs(
	ctx context.Context,
	client *accessgraph.ClientWithResponses,
	params accessgraph.ExecuteLogsQueryV1Params,
	maxResults int,
) (events []logmodels.AccessgraphStorageV1alphaEvent, truncated bool, err error) {
	var from, to string
	if params.StartTime != nil {
		from = params.StartTime.Format(time.RFC3339)
	}
	if params.EndTime != nil {
		to = params.EndTime.Format(time.RFC3339)
	}
	slog.DebugContext(ctx, "logs query", "query", strPtrToStr(params.Query), "from", from, "to", to, "max_results", maxResults)

	var (
		cursor *string
		pages  int
	)
	for {
		params.Iterator = cursor
		resp, err := doRequest(client.ExecuteLogsQueryV1WithResponse(ctx, &params))
		if err != nil {
			return nil, false, trace.Wrap(err)
		}
		pages++
		slog.DebugContext(ctx, "logs page fetched", "page", pages, "page_size", len(resp.JSON200.Data))
		events = append(events, resp.JSON200.Data...)
		if maxResults > 0 && len(events) >= maxResults {
			truncated = len(events) > maxResults || resp.JSON200.NextCursor != nil
			events = events[:maxResults]
			break
		}
		if resp.JSON200.NextCursor == nil {
			break
		}
		// Guard against a backend that returns a non-advancing cursor, which would otherwise spin forever.
		if cursor != nil && *resp.JSON200.NextCursor == *cursor {
			slog.DebugContext(ctx, "Access Graph cursor did not advance; stopping pagination", "cursor", *cursor)
			truncated = true
			break
		}
		cursor = resp.JSON200.NextCursor
	}
	slog.DebugContext(ctx, "logs fetch complete", "pages", pages, "events", len(events), "truncated", truncated)
	return events, truncated, nil
}

// strPtrToStr returns *s or "" if s is nil.
func strPtrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// dslClause returns `field:value` or `field:(v1 OR v2 ...)`; values are %q-quoted.
func dslClause(field string, values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%s:%q", field, values[0])
	default:
		parts := make([]string, len(values))
		for i, v := range values {
			parts[i] = fmt.Sprintf("%q", v)
		}
		return fmt.Sprintf("%s:(%s)", field, strings.Join(parts, " OR "))
	}
}

// writeOutput dispatches payload rendering: text invokes renderText; json/yaml marshal payload directly.
func writeOutput(w io.Writer, payload any, format string, renderText func(io.Writer) error) error {
	switch format {
	case teleport.Text:
		return trace.Wrap(renderText(w))
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(w, payload))
	case teleport.YAML:
		return trace.Wrap(utils.WriteYAML(w, payload))
	default:
		return trace.BadParameter("unknown format %q", format)
	}
}

// displayEventsText renders access-graph log events as a compact table.
func displayEventsText(out io.Writer, events []logmodels.AccessgraphStorageV1alphaEvent) error {
	if len(events) == 0 {
		_, err := fmt.Fprintln(out, "No events found.")
		return trace.Wrap(err)
	}

	table := asciitable.MakeTable([]string{
		"Time",
		"Identity",
		"Event Type",
		"Action",
		"Status",
		"Resource",
		"Source",
	})
	for _, ev := range events {
		identity := ev.Identity.Name
		if identity == "" {
			identity = ev.Identity.Id
		}
		resource := ev.Target.Resource
		if resource == "" && ev.Target.Id != "" {
			resource = ev.Target.Id
		}
		table.AddRow([]string{
			ev.Time.Format(time.RFC3339),
			identity,
			ev.EventType,
			ev.Action,
			ev.Status,
			resource,
			strings.TrimSpace(string(ev.EventSource)),
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
