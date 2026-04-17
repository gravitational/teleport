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
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	accessgraph "github.com/gravitational/access-graph/api/client"
	logmodels "github.com/gravitational/access-graph/api/client/models/logs"
	"github.com/gravitational/teleport"
	types "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type requestStateValue struct {
	target *types.RequestState
}

func (v requestStateValue) Set(s string) error {
	value, ok := types.RequestState_value[s]
	if !ok {
		return trace.BadParameter("invalid request state %q", s)
	}

	*v.target = types.RequestState(value)
	return nil
}

func (v requestStateValue) String() string {
	if v.target == nil {
		return ""
	}

	return v.target.String()
}

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

func parseTimeFilterValue(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts, nil
	}

	duration, err := parseRelativeDuration(s)
	if err != nil {
		return time.Time{}, trace.BadParameter("invalid time %q, expected RFC3339 or relative duration like 24h or 7d", s)
	}

	return now.Add(-duration), nil
}

// registerFormatFlag adds a `--format` flag (text/json/yaml) to cmd with the
// given default, storing the selected value in target.
func registerFormatFlag(cmd *kingpin.CmdClause, target *string, defaultFormat string) {
	cmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(defaultFormat).
		EnumVar(target, teleport.Text, teleport.JSON, teleport.YAML)
}

// registerTimeRangeFlags adds `--from` and `--to` flags to cmd, with the given
// `--from` default (e.g. "30d"). `--to` defaults to now at parse time.
func registerTimeRangeFlags(cmd *kingpin.CmdClause, from, to *time.Time, defaultFrom string) {
	cmd.Flag("from", fmt.Sprintf("Include activity at or after this time. (Examples: %s, 24h, 7d, Default: %s)", time.RFC3339, defaultFrom)).
		Default(defaultFrom).
		SetValue(timeValue{target: from})
	cmd.Flag("to", fmt.Sprintf("Include activity at or before this time. (Examples: %s, 24h, 7d, Default: now)", time.RFC3339)).
		Default(time.Now().Format(time.RFC3339)).
		SetValue(timeValue{target: to})
}

// writeOutput dispatches value to the right encoder for the given format. The
// textFn is called only for the text path so callers don't have to construct a
// text representation when emitting JSON/YAML.
func writeOutput(out io.Writer, value any, format string, textFn func(io.Writer) error) error {
	switch format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSON(out, value))
	case teleport.Text:
		return trace.Wrap(textFn(out))
	default:
		return trace.Wrap(utils.WriteYAML(out, value))
	}
}


// fetchAllLogs paginates an ExecuteLogsQueryV1 call until the cursor is
// exhausted and returns the accumulated events. The caller provides the base
// params; the iterator field is managed here.
func fetchAllLogs(
	ctx context.Context,
	client *accessgraph.ClientWithResponses,
	params accessgraph.ExecuteLogsQueryV1Params,
) ([]logmodels.AccessgraphStorageV1alphaEvent, error) {
	var (
		all    []logmodels.AccessgraphStorageV1alphaEvent
		cursor *string
		pages  int
	)
	for {
		params.Iterator = cursor
		resp, err := doRequest(client.ExecuteLogsQueryV1WithResponse(ctx, &params))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		pages++
		slog.DebugContext(ctx, "logs page fetched", "page", pages, "page_size", len(resp.JSON200.Data))
		all = append(all, resp.JSON200.Data...)
		if resp.JSON200.NextCursor == nil {
			break
		}
		cursor = resp.JSON200.NextCursor
	}
	slog.DebugContext(ctx, "logs fetch complete", "pages", pages, "events", len(all))
	return all, nil
}

// strPtrToStr returns the string pointed to by s, or "" if s is nil. Useful
// for OpenAPI-generated structs that represent optional strings as *string.
func strPtrToStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// getStringSlice extracts a []string from a decoded JSON map, handling both
// typed slices and the []any form produced by generic decoders. Returns nil
// when the key is absent, nil, or holds an unexpected type.
func getStringSlice(data map[string]any, key string) []string {
	raw, ok := data[key]
	if !ok || raw == nil {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		out := make([]string, len(values))
		copy(out, values)
		return out
	case []any:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if s, ok := value.(string); ok && s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

// quoteAll returns each value quoted for safe use in the logs DSL, where "@"
// and other characters would otherwise be interpreted as tokens.
func quoteAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("%q", v)
	}
	return out
}

// dslClause returns `field:value` for one value or `field:(v1 OR v2)` for many.
// Values should already be quoted as needed (see quoteAll).
func dslClause(field string, values []string) string {
	switch len(values) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%s:%s", field, values[0])
	default:
		return fmt.Sprintf("%s:(%s)", field, strings.Join(values, " OR "))
	}
}

func parseRelativeDuration(s string) (time.Duration, error) {
	if before, ok := strings.CutSuffix(s, "d"); ok {
		days, err := time.ParseDuration(before + "h")
		if err != nil {
			return 0, trace.Wrap(err)
		}

		return days * 24, nil
	}

	duration, err := time.ParseDuration(s)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return duration, nil
}
