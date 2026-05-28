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
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
)

// descriptionTextMaxLen caps Description in the detailed text table so a long
// write-up doesn't blow the row width out; JSON/YAML emit the full body.
const descriptionTextMaxLen = 80

type detectionsArgs struct {
	cmd *kingpin.CmdClause
	ls  detectionsListArgs
	get detectionsGetArgs

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
	c.initDetectionsGet(c.detections.cmd)
}

func (c *AccessGraphCommand) initDetectionsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List Identity Security detections.")

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
		if resp.JSON200 == nil {
			return nil, trace.Errorf("received nil json response from Access Graph API")
		}
		// Guard against a backend that returns a non-advancing cursor, which would otherwise spin forever.
		if cursor != nil && resp.JSON200.NextCursor != nil && *resp.JSON200.NextCursor == *cursor {
			slog.DebugContext(ctx, "Access Graph cursor did not advance; stopping pagination", "cursor", *cursor)
			return alerts, nil
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
	_, err := fmt.Fprintln(out, table.String())
	return trace.Wrap(err)
}

type detectionsGetArgs struct {
	cmd *kingpin.CmdClause
	id  string
}

func (c *AccessGraphCommand) initDetectionsGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get Identity Security detection details.")
	getCmd.Arg("id", "The detection ID to retrieve.").Required().StringVar(&c.detections.get.id)
	c.detections.get.cmd = getCmd
}

// detectionGetOutput is the payload for `detections get`.
type detectionGetOutput struct {
	Alert       accessgraph.SecurityAlert                  `json:"alert" yaml:"alert"`
	Events      []logmodels.AccessgraphStorageV1alphaEvent `json:"events" yaml:"events"`
	EventsError string                                     `json:"events_error,omitempty" yaml:"events_error,omitempty"`
}

// DetectionsGet executes `tctl detections get <id>`.
func (c *AccessGraphCommand) DetectionsGet(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	id, err := uuid.Parse(c.detections.get.id)
	if err != nil {
		return trace.BadParameter("invalid detection id %q: %v", c.detections.get.id, err)
	}
	resp, err := doRequest(client.GetAlertV1WithResponse(ctx, id))
	if err != nil {
		return trace.Wrap(err)
	}

	if resp.JSON200 == nil {
		return trace.Errorf("received nil json response from Access Graph API")
	}

	alert := resp.JSON200.Data

	// Non-fatal: alert detail is still useful; error surfaces in Log Entries.
	events, eventsErr := fetchAlertEvents(ctx, client, alert)

	out := detectionGetOutput{Alert: alert, Events: events}
	if eventsErr != nil {
		out.EventsError = eventsErr.Error()
	}
	return writeOutput(c.stdout, out, c.detections.format, func(w io.Writer) error {
		return displayDetectionText(w, alert, events, eventsErr)
	})
}

// eventsFetchErrorStyle paints the events-fetch warning yellow + bold.
var eventsFetchErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)

// fetchAlertEvents fetches the logs referenced by the alert's LogEntries.
func fetchAlertEvents(ctx context.Context, client *accessgraph.ClientWithResponses, a accessgraph.SecurityAlert) ([]logmodels.AccessgraphStorageV1alphaEvent, error) {
	if a.LogEntries == nil || len(*a.LogEntries) == 0 {
		return nil, nil
	}
	query := dslClause("uid", *a.LogEntries)
	order := accessgraph.Asc
	events, _, err := fetchAllLogs(ctx, client, accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		Order:     &order,
		StartTime: &a.StartTime,
		EndTime:   &a.EndTime,
	}, len(*a.LogEntries))
	return events, trace.Wrap(err)
}

// displayDetectionText renders the human-readable detail view for one alert.
func displayDetectionText(out io.Writer, a accessgraph.SecurityAlert, events []logmodels.AccessgraphStorageV1alphaEvent, eventsErr error) error {
	field := func(label, value string) {
		fmt.Fprintf(out, "%-19s%s\n", label+":", value)
	}
	field("ID", a.Id.String())
	field("Title", a.Title)
	field("Severity", string(a.Severity))
	field("Status", string(a.Status))
	if a.AffectedEntity != nil && a.AffectedEntity.Name != nil && *a.AffectedEntity.Name != "" {
		field("Affected Entity", *a.AffectedEntity.Name)
	}
	field("Type", a.Type)
	field("Source", string(a.Source))
	if a.ReportedBy != nil {
		field("Reported By", *a.ReportedBy)
	}
	if a.Tags != nil && len(*a.Tags) > 0 {
		field("Tags", strings.Join(*a.Tags, ", "))
	}
	field("Period", fmt.Sprintf("%s → %s", a.StartTime.Format(time.RFC3339), a.EndTime.Format(time.RFC3339)))
	field("Created", a.CreatedAt.Format(time.RFC3339))
	if a.UpdatedAt != nil {
		field("Updated", a.UpdatedAt.Format(time.RFC3339))
	}
	if a.Description != nil && *a.Description != "" {
		fmt.Fprintf(out, "\nDescription:\n%s\n", *a.Description)
	}
	if a.MitigationSteps != nil && len(*a.MitigationSteps) > 0 {
		fmt.Fprintln(out, "\nMitigation Steps:")
		for _, step := range *a.MitigationSteps {
			fmt.Fprintf(out, "  - %s\n", step)
		}
	}
	if eventsErr != nil || len(events) > 0 || (a.LogEntries != nil && len(*a.LogEntries) > 0) {
		fmt.Fprintln(out, "\nLog Entries:")
		switch {
		case eventsErr != nil:
			fmt.Fprintln(out, eventsFetchErrorStyle.Render(eventsErr.Error()))
		case len(events) == 0:
			fmt.Fprintln(out, "Not found.")
		default:
			if err := displayEventsText(out, events); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	if len(a.StatusChangeLogs) > 0 {
		fmt.Fprintln(out, "\nStatus Changes:")
		changes := asciitable.MakeTable([]string{"Time", "Status", "User", "Reason"})
		for _, log := range a.StatusChangeLogs {
			reason := ""
			if log.Reason != nil {
				reason = *log.Reason
			}
			changes.AddRow([]string{log.CreatedAt.Format(time.RFC3339), string(log.Status), log.User, reason})
		}
		fmt.Fprintln(out, changes.String())
	}
	return nil
}
