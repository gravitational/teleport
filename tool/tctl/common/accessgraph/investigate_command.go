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
	"github.com/gravitational/teleport"
	accessgraph "github.com/gravitational/teleport/lib/accessgraph/apiclient"
	logmodels "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/trace"
)

type investigateArgs struct {
	cmd      *kingpin.CmdClause
	user     investigateSubjectArgs
	resource investigateSubjectArgs

	// Shared across subcommands.
	from              time.Time
	to                time.Time
	limit             int
	format            string
	eventTypes        []string
	excludeEventTypes []string
}

type investigateSubjectArgs struct {
	cmd  *kingpin.CmdClause
	name string
}

func (c *AccessGraphCommand) initInvestigate(app *kingpin.Application) {
	investigateCmd := app.Command("investigate", "Investigate identity or resource activity.")
	registerTimeRangeFlags(investigateCmd, &c.investigate.from, &c.investigate.to, "7d")
	investigateCmd.Flag("limit", "Max events to return (0 for unlimited).").
		Default("100").
		IntVar(&c.investigate.limit)
	// TODO(accessgraph): support glob patterns (e.g. cert.*) once the access
	// graph logs endpoint can accept wildcard filters. The Athena backend
	// currently rejects the DSL's SIMILAR TO translation, and filtering
	// client-side was too slow to be usable.
	investigateCmd.Flag("event-type", "Include only events of these types (repeatable, e.g. --event-type session.start).").
		StringsVar(&c.investigate.eventTypes)
	investigateCmd.Flag("exclude-event-type", "Exclude events of these types (repeatable).").
		StringsVar(&c.investigate.excludeEventTypes)
	registerFormatFlag(investigateCmd, &c.investigate.format, teleport.YAML)
	c.investigate.cmd = investigateCmd

	c.investigate.user.cmd = investigateCmd.Command("user", "Investigate activity for a specific user.")
	c.investigate.user.cmd.Arg("name", "User name to investigate.").Required().StringVar(&c.investigate.user.name)

	c.investigate.resource.cmd = investigateCmd.Command("resource", "Investigate activity for a specific resource.")
	c.investigate.resource.cmd.Arg("name", "Resource name to investigate.").Required().StringVar(&c.investigate.resource.name)
}

// Investigate is kept as a placeholder so the current CLI dispatch keeps
// working; the real entry points are the user/resource subcommands below.
func (c *AccessGraphCommand) Investigate(ctx context.Context, args accessGraphServices) error {
	return trace.BadParameter("specify 'user' or 'resource' subcommand: tctl investigate user <name> | tctl investigate resource <name>")
}

// InvestigateUser executes `tctl investigate user <name>`.
//
// Uses identity_id — the logs DSL's `identity` field targets a struct column
// and fails at the Athena layer, but identity_id resolves to a scalar and
// matches the canonical identifier (email for users, generated id for bots).
func (c *AccessGraphCommand) InvestigateUser(ctx context.Context, args accessGraphServices) error {
	return c.runInvestigate(ctx, args, dslClause("identity_id", quoteAll([]string{c.investigate.user.name})))
}

// InvestigateResource executes `tctl investigate resource <name>`.
func (c *AccessGraphCommand) InvestigateResource(ctx context.Context, args accessGraphServices) error {
	return c.runInvestigate(ctx, args, dslClause("resource", quoteAll([]string{c.investigate.resource.name})))
}

func (c *AccessGraphCommand) runInvestigate(ctx context.Context, args accessGraphServices, subjectClause string) error {
	parts := []string{subjectClause}
	if clause := dslClause("event_type", quoteAll(c.investigate.eventTypes)); clause != "" {
		parts = append(parts, clause)
	}
	if clause := dslClause("event_type", quoteAll(c.investigate.excludeEventTypes)); clause != "" {
		parts = append(parts, "NOT "+clause)
	}
	query := strings.Join(parts, " AND ")

	order := accessgraph.Desc
	params := accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &c.investigate.from,
		EndTime:   &c.investigate.to,
		Order:     &order,
	}
	if c.investigate.limit > 0 {
		params.Limit = &c.investigate.limit
	}

	events, err := fetchAllLogs(ctx, args.accessGraph, params)
	if err != nil {
		return trace.Wrap(err)
	}
	if c.investigate.limit > 0 && len(events) > c.investigate.limit {
		events = events[:c.investigate.limit]
	}

	return writeOutput(c.stdout, events, c.investigate.format, func(w io.Writer) error {
		return displayInvestigateText(w, events, c.investigate.from, c.investigate.to)
	})
}

func displayInvestigateText(out io.Writer, events []logmodels.AccessgraphStorageV1alphaEvent, from, to time.Time) error {
	fmt.Fprintf(out, "Period: %s → %s\n", from.Format(time.RFC3339), to.Format(time.RFC3339))
	fmt.Fprintf(out, "Events: %d\n\n", len(events))
	return displayEventsText(out, events)
}

// displayEventsText renders a list of access-graph log events as a compact
// table. Callers that show their own header (e.g. detections get) can use this
// directly instead of displayInvestigateText.
//
// TODO(accessgraph): produce a human-readable rendering per event type. Today
// we show a generic row for every event; many event types carry useful
// details in EventData that would make this significantly more informative.
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
