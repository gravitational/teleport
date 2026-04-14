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
	models "github.com/gravitational/access-graph/api/client/models/logs"
	"github.com/gravitational/teleport"
	types "github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

type accessRequestsArgs struct {
	cmd *kingpin.CmdClause
	ls  accessRequestsListArgs
}

type accessRequestsListArgs struct {
	cmd *kingpin.CmdClause

	// General filters
	kind      string
	state     types.RequestState
	requester string
	approver  string

	// Date filters
	from time.Time
	to   time.Time

	// Meta filters
	unused bool

	// Output control
	limit  int
	format string
}

func (c *AccessGraphCommand) initAccessRequests(app *kingpin.Application) {
	c.accessRequests.cmd = app.Command("access-requests", "Review access requests and approvals.").Hidden()
	c.initAccessRequestsList(c.accessRequests.cmd)
}

func (c *AccessGraphCommand) initAccessRequestsList(parent *kingpin.CmdClause) {
	lsCmd := parent.Command("ls", "List access requests.")
	lsCmd.Flag("kind", "Filter for a specific kind of access request. (Example: kube_cluster, database, role)").
		StringVar(&c.accessRequests.ls.kind)
	lsCmd.Flag("state", "Filter by request state. (Values: NONE, PENDING, APPROVED, DENIED, PROMOTED)").
		SetValue(requestStateValue{target: &c.accessRequests.ls.state})
	lsCmd.Flag("user", "Filter by the Teleport user who created the request. (Example: alice)").
		StringVar(&c.accessRequests.ls.requester)
	lsCmd.Flag("approver", "Filter by the Teleport user who approved the request. (Example: bob)").
		StringVar(&c.accessRequests.ls.approver)
	lsCmd.Flag("limit", "Limit the number of access requests returned.").
		Default("50").
		IntVar(&c.accessRequests.ls.limit)
	lsCmd.Flag("from", fmt.Sprintf("Filter requests created at or after this time. (Examples: %s, 24h, 7d, Default: 30d)", time.RFC3339)).
		Default("30d").
		SetValue(timeValue{target: &c.accessRequests.ls.from})
	lsCmd.Flag("to", fmt.Sprintf("Filter requests created at or before this time. (Examples: %s, 24h, 7d, Default: now)", time.RFC3339)).
		Default(time.Now().Format(time.RFC3339)).
		SetValue(timeValue{target: &c.accessRequests.ls.to})
	lsCmd.Flag("unused", "Filter for requests that were approved but not used.").
		BoolVar(&c.accessRequests.ls.unused)
	lsCmd.Flag("format", "Output format. (Values: text, json, yaml)").
		Default(teleport.YAML).
		EnumVar(&c.accessRequests.ls.format, teleport.Text, teleport.JSON, teleport.YAML)

	c.accessRequests.ls.cmd = lsCmd
}

// AccessRequestsList executes `tctl access-requests ls`.
func (c *AccessGraphCommand) AccessRequestsList(ctx context.Context, args accessGraphServices) error {
	query := constructAccessRequestsListQuery(c.accessRequests.ls)

	resp, err := doRequest(args.accessGraph.ExecuteLogsQueryV1WithResponse(ctx, &query))

	if err != nil {
		return fmt.Errorf("failed to execute access requests query: %w", err)
	}

	return displayAccessRequests(c.stdout, resp.JSON200.Data, c.accessRequests.ls.format)
}

func constructAccessRequestsListQuery(args accessRequestsListArgs) accessgraph.ExecuteLogsQueryV1Params {
	queryParts := []string{"event_type:(access_request.create OR access_request.review OR access_request.expire OR access_request.update OR access_request.delete)"}
	if args.kind != "" {
		queryParts = append(queryParts, fmt.Sprintf("target_kind:%s", args.kind))
	}
	if args.state != types.RequestState_NONE {
		queryParts = append(queryParts, fmt.Sprintf("status:%s", args.state.String()))
	}
	if args.requester != "" {
		queryParts = append(queryParts, fmt.Sprintf("identity:%s", args.requester))
	}
	if args.approver != "" {
		queryParts = append(queryParts, fmt.Sprintf("approver:%s", args.approver))
	}
	query := strings.Join(queryParts, " AND ")

	fmt.Println(query)

	return accessgraph.ExecuteLogsQueryV1Params{
		Query:     &query,
		StartTime: &args.from,
		EndTime:   &args.to,
		Limit:     &args.limit,
	}

}
func displayAccessRequests(out io.Writer, logs []models.AccessgraphStorageV1alphaEvent, format string) error {
	switch format {
	case teleport.JSON:
		return trace.Wrap(utils.WriteJSONArray(out, logs))
	case teleport.Text:
		return trace.Wrap(displayAccessRequestsText(out, logs))
	default:
		return trace.Wrap(utils.WriteYAML(out, logs))
	}
}

func displayAccessRequestsText(out io.Writer, logs []models.AccessgraphStorageV1alphaEvent) error {
	table := asciitable.MakeTable([]string{"ID", "User", "State"})

	for _, log := range logs {
		ev, err := log.EventData.AsTeleportAuditLog()
		if err != nil {
			return trace.Wrap(err, "failed to parse event data for log ID %q", log.Uuid)
		}
		table.AddRow([]string{
			log.Target.Resource,
			log.Identity.Name,
			string(ev["state"].(string)),
		})
	}
	_, err := fmt.Fprintln(out, table.AsBuffer().String())
	return trace.Wrap(err)
}
