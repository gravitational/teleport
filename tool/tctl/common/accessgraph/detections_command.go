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
	"os"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"golang.org/x/term"

	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tctl/common/editor"
)

// descriptionTextMaxLen caps Description in the detailed text table so a long
// write-up doesn't blow the row width out; JSON/YAML emit the full body.
const descriptionTextMaxLen = 80

type detectionsArgs struct {
	cmd *kingpin.CmdClause
	ls  detectionsListArgs
	get detectionsGetArgs

	// Status-change subcommands, one per lifecycle verb.
	triage  detectionSetStatusArgs
	resolve detectionSetStatusArgs
	close   detectionSetStatusArgs

	// editor is used by tests to inject the editing mechanism so that
	// different scenarios can be asserted.
	editor func(ctx context.Context, filename string) error
}

type detectionsListArgs struct {
	cmd *kingpin.CmdClause

	// Date filters
	from time.Time
	to   time.Time

	// General filters
	status   []string
	source   []string
	typ      []string
	severity []string

	// limit caps the total number of alerts returned across paginated calls.
	limit int

	// detailed expands the ls command to carry extra columns
	detailed bool

	// Output format
	format string
}

// Output and time-window flags are declared per subcommand rather than on the
// `detections` parent so the status verbs don't advertise options they ignore.
func (c *AccessGraphCommand) initDetections(app *kingpin.Application) {
	detectionsCmd := app.Command("detections", "Investigate security detections and anomalies.")
	c.detections.cmd = detectionsCmd

	c.initDetectionsList(detectionsCmd)
	c.initDetectionsGet(detectionsCmd)
	c.initDetectionsSetStatus(detectionsCmd, "triage", "Mark a detection as triaged.", &c.detections.triage)
	c.initDetectionsSetStatus(detectionsCmd, "resolve", "Mark a detection as resolved.", &c.detections.resolve)
	c.initDetectionsSetStatus(detectionsCmd, "close", "Mark a detection as closed.", &c.detections.close)
}

func (c *AccessGraphCommand) initDetectionsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List Identity Security detections.")

	lsCmd.Flag("from", fmt.Sprintf("Include activity at or after this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: 30d)", time.RFC3339, time.DateOnly)).
		Default("30d").
		SetValue(timeValue{target: &c.detections.ls.from})
	lsCmd.Flag("to", fmt.Sprintf("Include activity at or before this time. (Examples: %s, %s, 24h, 7d; negative durations like -1h are future-relative. Default: now)", time.RFC3339, time.DateOnly)).
		Default("now").
		SetValue(timeValue{target: &c.detections.ls.to})
	lsCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.detections.ls.format, teleport.Text, teleport.JSON, teleport.YAML)
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
	if err := validateTimeWindow(c.detections.ls.from, c.detections.ls.to); err != nil {
		return trace.Wrap(err)
	}
	params := constructAlertsListQuery(c.detections.ls)
	alerts, err := fetchAlerts(ctx, client, params, c.detections.ls.limit)
	if err != nil {
		return trace.Wrap(err)
	}
	return displayDetections(c.stdout, alerts, c.detections.ls.format, c.detections.ls.detailed)
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

func constructAlertsListQuery(args detectionsListArgs) accessgraph.ListAlertsV1Params {
	var queryParts []string
	for field, values := range map[string][]string{
		"status":   args.status,
		"source":   args.source,
		"type":     args.typ,
		"severity": args.severity,
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
		utils.EscapeControl(string(a.Status)),
		a.CreatedAt.Format(time.RFC3339),
		utils.EscapeControl(string(a.Source)),
		utils.EscapeControl(a.Title),
		utils.EscapeControl(string(a.Severity)),
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
		utils.EscapeControl(strPtrToStr(a.ReportedBy)),
		utils.EscapeControl(a.Type),
		utils.EscapeControl(formatAffectedEntity(a)),
		utils.EscapeControl(tags),
		utils.EscapeControl(description),
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

	// Output format
	format string
}

func (c *AccessGraphCommand) initDetectionsGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get Identity Security detection details.")
	getCmd.Arg("id", "The detection ID to retrieve.").Required().StringVar(&c.detections.get.id)
	getCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.Text).
		EnumVar(&c.detections.get.format, teleport.Text, teleport.JSON, teleport.YAML)
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
	return writeOutput(c.stdout, out, c.detections.get.format, func(w io.Writer) error {
		return displayDetectionText(w, alert, events, eventsErr)
	})
}

// warningStyle paints the events-fetch warning yellow + bold.
var warningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)

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
		fmt.Fprintf(out, "%-19s%s\n", label+":", utils.EscapeControl(value))
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
		fmt.Fprintf(out, "\nDescription:\n%s\n", utils.AllowWhitespace(*a.Description))
	}
	if a.MitigationSteps != nil && len(*a.MitigationSteps) > 0 {
		fmt.Fprintln(out, "\nMitigation Steps:")
		for _, step := range *a.MitigationSteps {
			fmt.Fprintf(out, "  - %s\n", utils.AllowWhitespace(step))
		}
	}
	if eventsErr != nil || len(events) > 0 || (a.LogEntries != nil && len(*a.LogEntries) > 0) {
		fmt.Fprintln(out, "\nLog Entries:")
		switch {
		case eventsErr != nil:
			fmt.Fprintln(out, warningStyle.Render(utils.AllowWhitespace(eventsErr.Error())))
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
			changes.AddRow([]string{
				log.CreatedAt.Format(time.RFC3339),
				utils.EscapeControl(string(log.Status)),
				utils.EscapeControl(log.User),
				utils.EscapeControl(reason),
			})
		}
		fmt.Fprintln(out, changes.String())
	}
	return nil
}

// detectionSetStatusArgs backs the triage/resolve/close subcommands.
type detectionSetStatusArgs struct {
	cmd        *kingpin.CmdClause
	id         string
	reason     string
	allowEmpty bool
}

func (c *AccessGraphCommand) initDetectionsSetStatus(parent *kingpin.CmdClause, verb, help string, target *detectionSetStatusArgs) {
	cmd := parent.Command(verb, help)
	cmd.Arg("id", "The detection ID to update.").Required().StringVar(&target.id)
	cmd.Flag("reason", "Reason for the status change. If omitted, an editor is opened to collect one.").
		Short('r').
		StringVar(&target.reason)
	cmd.Flag("allow-empty", "Change the status without a reason instead of prompting for one.").
		BoolVar(&target.allowEmpty)
	target.cmd = cmd
}

// DetectionsTriage executes `tctl detections triage <id>`.
func (c *AccessGraphCommand) DetectionsTriage(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	return c.detectionsSetStatus(ctx, client, c.detections.triage, accessgraph.Triaged)
}

// DetectionsResolve executes `tctl detections resolve <id>`.
func (c *AccessGraphCommand) DetectionsResolve(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	return c.detectionsSetStatus(ctx, client, c.detections.resolve, accessgraph.Resolved)
}

// DetectionsClose executes `tctl detections close <id>`.
func (c *AccessGraphCommand) DetectionsClose(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	return c.detectionsSetStatus(ctx, client, c.detections.close, accessgraph.Closed)
}

// detectionsSetStatus writes status to the detection identified by args.id.
func (c *AccessGraphCommand) detectionsSetStatus(ctx context.Context, client *accessgraph.ClientWithResponses, args detectionSetStatusArgs, status accessgraph.AlertStatus) error {
	id, err := uuid.Parse(args.id)
	if err != nil {
		return trace.BadParameter("invalid detection id %q: %v", args.id, err)
	}

	reason, err := c.resolveReason(ctx, args, status)
	if err != nil {
		return trace.Wrap(err)
	}

	body := accessgraph.PutAlertStatusRequest{Status: status}
	if reason != "" {
		body.Reason = &reason
	}

	// UpdateAlertStatusV1Response has no success body; doRequest's status check is the only success signal.
	if _, err := doRequest(client.UpdateAlertStatusV1WithResponse(ctx, id, body)); err != nil {
		return trace.Wrap(err)
	}

	_, err = fmt.Fprintf(c.stdout, "Detection %s set to %s.\n", id, status)
	return trace.Wrap(err)
}

// resolveReason returns the --reason flag, "" when --allow-empty is set, or an interactively collected reason.
func (c *AccessGraphCommand) resolveReason(ctx context.Context, args detectionSetStatusArgs, status accessgraph.AlertStatus) (string, error) {
	if reason := strings.TrimSpace(args.reason); reason != "" {
		return reason, nil
	}
	if args.allowEmpty {
		return "", nil
	}

	reason, err := c.collectReason(ctx, args.id, status)
	if err != nil {
		return "", trace.Wrap(err)
	}
	if reason == "" {
		return "", trace.BadParameter("empty reason, aborting; pass --allow-empty to change the status without one")
	}
	return reason, nil
}

// reasonTemplate seeds the editor buffer, leaving the cursor on a blank first line.
const reasonTemplate = `
# Please enter a reason for setting detection %s to %q.
# Lines starting with '#' are ignored, and an empty reason aborts the change.
`

// collectReason opens the editor on a seeded temporary file and returns what
// the user typed, minus the seeded comments.
func (c *AccessGraphCommand) collectReason(ctx context.Context, id string, status accessgraph.AlertStatus) (string, error) {
	f, err := os.CreateTemp("", "teleport-detection-reason*.txt")
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer os.Remove(f.Name())

	if _, err := fmt.Fprintf(f, reasonTemplate, id, status); err != nil {
		_ = f.Close()
		return "", trace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return "", trace.Wrap(err)
	}

	if err := c.runReasonEditor(ctx, f.Name()); err != nil {
		return "", trace.Wrap(err)
	}

	edited, err := os.ReadFile(f.Name())
	if err != nil {
		return "", trace.Wrap(err)
	}
	return stripComments(string(edited)), nil
}

// runReasonEditor opens filename in the user's editor, which requires a terminal.
func (c *AccessGraphCommand) runReasonEditor(ctx context.Context, filename string) error {
	if c.detections.editor != nil {
		return trace.Wrap(c.detections.editor(ctx, filename))
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return trace.BadParameter("no terminal available to prompt for a reason; pass --reason/-r or --allow-empty")
	}
	return trace.Wrap(editor.Run(ctx, filename))
}

// stripComments drops whole-line '#' comments and trims the result.
func stripComments(s string) string {
	var kept []string
	for line := range strings.SplitSeq(s, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
