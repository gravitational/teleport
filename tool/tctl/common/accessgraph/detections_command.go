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
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	"github.com/gravitational/teleport/lib/asciitable"
)

// descriptionTextMaxLen caps Description in the detailed text table so a long
// write-up doesn't blow the row width out; JSON/YAML emit the full body.
const descriptionTextMaxLen = 80

type detectionsArgs struct {
	cmd *kingpin.CmdClause
	ls  detectionsListArgs

	// Date filters
	from time.Time
	to   time.Time

	// Output format
	format string
}

type detectionsListArgs struct {
	cmd *kingpin.CmdClause

	// General filters
	status   []string
	source   []string
	typ      []string
	severity []string

	// limit caps the total number of alerts returned across paginated calls.
	limit int

	// detailed expands the ls command to carry extra columns
	detailed bool
}

func (c *AccessGraphCommand) initDetections(app *kingpin.Application) {
	detectionsCmd := app.Command("detections", "Investigate security detections and anomalies.").Hidden()
	detectionsCmd.Flag("from", fmt.Sprintf("Include activity at or after this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: 30d)", time.RFC3339, time.DateOnly)).
		Default("30d").
		SetValue(timeValue{target: &c.detections.from})
	detectionsCmd.Flag("to", fmt.Sprintf("Include activity at or before this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: now)", time.RFC3339, time.DateOnly)).
		Default("now").
		SetValue(timeValue{target: &c.detections.to})
	detectionsCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.detections.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.detections.cmd = detectionsCmd

	c.initDetectionsList(c.detections.cmd)
}

func (c *AccessGraphCommand) initDetectionsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List security detections.")

	lsCmd.Flag("status", "Filter detections by status (Values: in_progress, triaged, resolved, closed). Default: in_progress, triaged.").
		AllowDuplicate().
		Default("in_progress", "triaged").
		EnumsVar(&c.detections.ls.status, "in_progress", "triaged", "resolved", "closed")
	lsCmd.Flag("source", "Filter detections by source.").
		AllowDuplicate().
		StringsVar(&c.detections.ls.source)
	lsCmd.Flag("type", "Filter detections by type.").
		AllowDuplicate().
		StringsVar(&c.detections.ls.typ)
	lsCmd.Flag("severity", "Filter detections by severity (Values: low, medium, high, critical).").
		AllowDuplicate().
		EnumsVar(&c.detections.ls.severity, "low", "medium", "high", "critical")
	lsCmd.Flag("detailed", "Include extra columns (Reported By, Type, Affected Entity, Tags, Description, Start Time, End Time, Updated) in text output.").
		BoolVar(&c.detections.ls.detailed)
	lsCmd.Flag("limit", "Maximum number of detections to return.").
		Default("100").
		IntVar(&c.detections.ls.limit)
	c.detections.ls.cmd = lsCmd
}

// DetectionsList executes `tctl detections ls`.
func (c *AccessGraphCommand) DetectionsList(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	if err := validateTimeWindow(c.detections.from, c.detections.to); err != nil {
		return trace.Wrap(err)
	}
	params := constructAlertsListQuery(c.detections)
	alerts, err := fetchAlerts(ctx, client, params, c.detections.ls.limit)
	if err != nil {
		return trace.Wrap(err)
	}
	return displayDetections(c.stdout, alerts, c.detections.format, c.detections.ls.detailed)
}

// fetchAlerts paginates ListAlertsV1 until limit alerts have been collected or
// the server signals no more pages. The final slice is trimmed to limit.
func fetchAlerts(
	ctx context.Context,
	client *accessgraph.ClientWithResponses,
	params accessgraph.ListAlertsV1Params,
	limit int,
) ([]accessgraph.SecurityAlert, error) {
	var (
		alerts []accessgraph.SecurityAlert
		cursor *string
	)
	for {
		params.NextCursor = cursor
		resp, err := doRequest(client.ListAlertsV1WithResponse(ctx, &params))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		alerts = append(alerts, resp.JSON200.Data...)
		if limit > 0 && len(alerts) >= limit {
			return alerts[:limit], nil
		}
		if resp.JSON200.NextCursor == nil {
			return alerts, nil
		}
		cursor = resp.JSON200.NextCursor
	}
}

func constructAlertsListQuery(args detectionsArgs) accessgraph.ListAlertsV1Params {
	var queryParts []string
	for field, values := range map[string][]string{
		"status":   args.ls.status,
		"source":   args.ls.source,
		"type":     args.ls.typ,
		"severity": args.ls.severity,
	} {
		if clause := dslClause(field, values); clause != "" {
			queryParts = append(queryParts, clause)
		}
	}
	query := strings.Join(queryParts, " AND ")
	return accessgraph.ListAlertsV1Params{
		StartTime: &args.from,
		EndTime:   &args.to,
		Query:     &query,
	}
}

func displayDetections(out io.Writer, alerts []accessgraph.SecurityAlert, format string, detailed bool) error {
	if alerts == nil {
		alerts = []accessgraph.SecurityAlert{}
	}
	return writeOutput(out, alerts, format, func(w io.Writer) error {
		return displayDetectionsText(w, alerts, detailed)
	})
}

// detectionRowHeaders returns the column titles for the text row schema.
// Kept aligned with detectionRow so the two never drift out of step.
func detectionRowHeaders(detailed bool) []string {
	headers := []string{
		"ID",
		"Status",
		"Date",
		"Source",
		"Alert",
		"Severity",
	}
	if detailed {
		headers = append(headers,
			"Reported By",
			"Type",
			"Affected Entity",
			"Tags",
			"Description",
			"Start Time",
			"End Time",
			"Updated",
		)
	}
	return headers
}

// detectionRow renders a single SecurityAlert as a row matching detectionRowHeaders.
func detectionRow(a accessgraph.SecurityAlert, detailed bool) []string {
	row := []string{
		a.Id.String(),
		string(a.Status),
		a.CreatedAt.Format(time.RFC3339),
		string(a.Source),
		a.Title,
		string(a.Severity),
	}
	if !detailed {
		return row
	}

	description := ""
	if a.Description != nil {
		description = truncateOneLine(*a.Description, descriptionTextMaxLen)
	}
	tags := ""
	if a.Tags != nil {
		tags = strings.Join(*a.Tags, ", ")
	}
	updated := ""
	if a.UpdatedAt != nil {
		updated = a.UpdatedAt.Format(time.RFC3339)
	}
	row = append(row,
		strPtrToStr(a.ReportedBy),
		a.Type,
		formatAffectedEntity(a),
		tags,
		description,
		a.StartTime.Format(time.RFC3339),
		a.EndTime.Format(time.RFC3339),
		updated,
	)
	return row
}

// formatAffectedEntity returns the AffectedEntity name, or an empty string
// when no name is set.
func formatAffectedEntity(a accessgraph.SecurityAlert) string {
	if a.AffectedEntity == nil {
		return ""
	}
	return strPtrToStr(a.AffectedEntity.Name)
}

// truncateOneLine collapses newlines to spaces and clips to max runes with an ellipsis suffix.
func truncateOneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(runes[:max-1]) + "…"
}

func displayDetectionsText(out io.Writer, alerts []accessgraph.SecurityAlert, detailed bool) error {
	headers := detectionRowHeaders(detailed)
	rows := make([][]string, 0, len(alerts))
	for _, alert := range alerts {
		rows = append(rows, detectionRow(alert, detailed))
	}
	table := asciitable.MakeTable(headers, rows...)
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
