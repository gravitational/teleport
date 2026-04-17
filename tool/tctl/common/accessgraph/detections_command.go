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
	accessgraph "github.com/gravitational/access-graph/api/client"
	logmodels "github.com/gravitational/access-graph/api/client/models/logs"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
	"github.com/google/uuid"
)

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
}

type detectionsGetArgs struct {
	cmd *kingpin.CmdClause
	id  string
}

func (c *AccessGraphCommand) initDetections(app *kingpin.Application) {
	detectionsCmd := app.Command("detections", "Investigate security detections and anomalies.").Hidden()
	registerTimeRangeFlags(detectionsCmd, &c.detections.from, &c.detections.to, "30d")
	registerFormatFlag(detectionsCmd, &c.detections.format, teleport.Text)
	c.detections.cmd = detectionsCmd

	c.initDetectionsList(c.detections.cmd)
	c.initDetectionsGet(c.detections.cmd)
}

func (c *AccessGraphCommand) initDetectionsGet(parent *kingpin.CmdClause) {
	getCmd := parent.Command("get", "Get details for a specific detection.")
	getCmd.Arg("id", "The detection ID to retrieve.").Required().StringVar(&c.detections.get.id)
	c.detections.get.cmd = getCmd
}

func (c *AccessGraphCommand) initDetectionsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List security detections.")

	lsCmd.Flag("status", "Filter detections by status (Values: in_progress, triaged, resolved, closed).").
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
		Default("low", "medium", "high", "critical").
		EnumsVar(&c.detections.ls.severity, "low", "medium", "high", "critical")
	c.detections.ls.cmd = lsCmd

}

// DetectionsList executes `tctl detections ls`.
func (c *AccessGraphCommand) DetectionsList(ctx context.Context, args accessGraphServices) error {
	query := constructAlertsListQuery(c.detections)

	resp, err := doRequest(args.accessGraph.ListAlertsV1WithResponse(ctx, &query))

	if err != nil {
		return trace.Wrap(err)
	}

	return displayDetections(c.stdout, resp.JSON200.Data, c.detections.format)
}

// DetectionGetOutput is the response payload for `detections get`. Events are
// fetched for the alert's log entries so JSON/YAML callers don't get stuck
// with bare UUIDs.
type DetectionGetOutput struct {
	Alert  accessgraph.SecurityAlert                  `json:"alert" yaml:"alert"`
	Events []logmodels.AccessgraphStorageV1alphaEvent `json:"events" yaml:"events"`
}

// DetectionsGet executes `tctl detections get <id>`.
func (c *AccessGraphCommand) DetectionsGet(ctx context.Context, args accessGraphServices) error {
	id, err := uuid.Parse(c.detections.get.id)
	if err != nil {
		return trace.BadParameter("invalid detection id %q: %v", c.detections.get.id, err)
	}
	resp, err := doRequest(args.accessGraph.GetAlertV1WithResponse(ctx, id))
	if err != nil {
		return trace.Wrap(err)
	}
	alert := resp.JSON200.Data

	events, err := fetchAlertEvents(ctx, args, alert)
	if err != nil {
		return trace.Wrap(err)
	}

	out := DetectionGetOutput{Alert: alert, Events: events}
	return writeOutput(c.stdout, out, c.detections.format, func(w io.Writer) error {
		return displayDetectionText(w, alert, events)
	})
}

// fetchAlertEvents queries the logs referenced by the alert's LogEntries. Uses
// the alert's own period as the time window and returns an empty slice when
// there are no referenced entries.
func fetchAlertEvents(ctx context.Context, args accessGraphServices, a accessgraph.SecurityAlert) ([]logmodels.AccessgraphStorageV1alphaEvent, error) {
	if a.LogEntries == nil || len(*a.LogEntries) == 0 {
		return nil, nil
	}
	query := dslClause("uid", quoteAll(*a.LogEntries))
	order := accessgraph.Asc
	return fetchAllLogs(ctx, args.accessGraph, accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &a.StartTime,
		EndTime:   &a.EndTime,
		Order:     &order,
	})
}

func displayDetectionText(out io.Writer, a accessgraph.SecurityAlert, events []logmodels.AccessgraphStorageV1alphaEvent) error {
	fmt.Fprintf(out, "ID:         %s\n", a.Id.String())
	fmt.Fprintf(out, "Title:      %s\n", a.Title)
	fmt.Fprintf(out, "Type:       %s\n", a.Type)
	fmt.Fprintf(out, "Severity:   %s\n", a.Severity)
	fmt.Fprintf(out, "Status:     %s\n", a.Status)
	fmt.Fprintf(out, "Source:     %s\n", a.Source)
	if a.ReportedBy != nil {
		fmt.Fprintf(out, "Reported:   %s\n", *a.ReportedBy)
	}
	fmt.Fprintf(out, "Period:     %s → %s\n", a.StartTime.Format(time.RFC3339), a.EndTime.Format(time.RFC3339))
	fmt.Fprintf(out, "Created:    %s\n", a.CreatedAt.Format(time.RFC3339))
	if a.UpdatedAt != nil {
		fmt.Fprintf(out, "Updated:    %s\n", a.UpdatedAt.Format(time.RFC3339))
	}
	if a.AffectedEntity != nil {
		name, entType := "", ""
		if a.AffectedEntity.Name != nil {
			name = *a.AffectedEntity.Name
		}
		if a.AffectedEntity.Type != nil {
			entType = *a.AffectedEntity.Type
		}
		fmt.Fprintf(out, "Affected:   %s [%s]\n", name, entType)
	}
	if a.Tags != nil && len(*a.Tags) > 0 {
		fmt.Fprintf(out, "Tags:       %s\n", strings.Join(*a.Tags, ", "))
	}
	if a.Description != nil && *a.Description != "" {
		fmt.Fprintf(out, "\nDescription:\n%s\n", *a.Description)
	}
	if a.MitigationSteps != nil && len(*a.MitigationSteps) > 0 {
		fmt.Fprintln(out, "\nMitigation Steps:")
		for _, step := range *a.MitigationSteps {
			fmt.Fprintf(out, "  • %s\n", step)
		}
	}
	if len(events) > 0 || (a.LogEntries != nil && len(*a.LogEntries) > 0) {
		fmt.Fprintln(out, "\nLog Entries:")
		if err := displayEventsText(out, events); err != nil {
			return trace.Wrap(err)
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
		fmt.Fprintln(out, changes.AsBuffer().String())
	}
	return nil
}

func constructAlertsListQuery(args detectionsArgs) accessgraph.ListAlertsV1Params {
	var queryParts []string
	for field, values := range map[string][]string{
		"status":   args.ls.status,
		"source":   args.ls.source,
		"type":     args.ls.typ,
		"severity": args.ls.severity,
	} {
		if clause := dslClause(field, quoteAll(values)); clause != "" {
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

func displayDetections(out io.Writer, alerts []accessgraph.SecurityAlert, format string) error {
	if alerts == nil {
		alerts = []accessgraph.SecurityAlert{}
	}
	return writeOutput(out, alerts, format, func(w io.Writer) error {
		return displayDetectionsText(w, alerts)
	})
}

func displayDetectionsText(out io.Writer, alerts []accessgraph.SecurityAlert) error {
	table := asciitable.MakeTable([]string{
		"ID",
		"Source",
		"Type",
		"Severity",
		"Status",
		"Start Time",
		"End Time",
	})

	for _, alert := range alerts {
		table.AddRow([]string{
			alert.Id.String(),
			string(alert.Source),
			alert.Type,
			string(alert.Severity),
			string(alert.Status),
			alert.StartTime.Format(time.RFC3339),
			alert.EndTime.Format(time.RFC3339),
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
