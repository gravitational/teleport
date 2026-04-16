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
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

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
}

func (c *AccessGraphCommand) initDetections(app *kingpin.Application) {
	detectionsCmd := app.Command("detections", "Investigate security detections and anomalies.").Hidden()

	detectionsCmd.Flag("from", fmt.Sprintf("Filter requests created at or after this time. (Examples: %s, 24h, 7d, Default: 30d)", time.RFC3339)).
		Default("30d").
		SetValue(timeValue{target: &c.detections.from})
	detectionsCmd.Flag("to", fmt.Sprintf("Filter requests created at or before this time. (Examples: %s, 24h, 7d, Default: now)", time.RFC3339)).
		Default(time.Now().Format(time.RFC3339)).
		SetValue(timeValue{target: &c.detections.to})
	detectionsCmd.Flag("format", "Output format: text, json, yaml.").
		Default(teleport.Text).
		EnumVar(&c.detections.format, teleport.Text, teleport.JSON, teleport.YAML)
	c.detections.cmd = detectionsCmd

	c.initDetectionsList(c.detections.cmd)
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

func constructAlertsListQuery(args detectionsArgs) accessgraph.ListAlertsV1Params {
	queryParts := []string{}
	if len(args.ls.status) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("status:(%s)", strings.Join(args.ls.status, " OR ")))
	}
	if len(args.ls.source) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("source:(%s)", strings.Join(args.ls.source, " OR ")))
	}
	if len(args.ls.typ) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("type:(%s)", strings.Join(args.ls.typ, " OR ")))
	}
	if len(args.ls.severity) > 0 {
		queryParts = append(queryParts, fmt.Sprintf("severity:(%s)", strings.Join(args.ls.severity, " OR ")))
	}
	query := strings.Join(queryParts, " AND ")
	return accessgraph.ListAlertsV1Params{
		StartTime: &args.from,
		EndTime:   &args.to,
		Query:     &query,
	}
}

func displayDetections(out io.Writer, alerts []accessgraph.SecurityAlert, format string) error {
	switch format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(out, alerts))
	case teleport.Text:
		return trace.Wrap(displayDetectionsText(out, alerts))
	default:
		return trace.Wrap(utils.WriteYAML(out, alerts))
	}
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
