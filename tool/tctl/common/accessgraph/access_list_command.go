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

	"github.com/alecthomas/kingpin/v2"
	accessclient "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
)

const defaultAccessGraphListQuery = "SELECT * FROM nodes"

func (c *AccessGraphCommand) initAccessList(parent *kingpin.CmdClause) {
	c.access.ls.cmd = parent.Command("ls", "List Access Graph resources using the query API.")
	c.access.ls.cmd.Flag("query", "SQL query to send to the Access Graph query API.").
		Default(defaultAccessGraphListQuery).
		StringVar(&c.access.ls.query)
}

// AccessList executes `tctl access ls`.
func (c *AccessGraphCommand) AccessList(ctx context.Context, args accessGraphServices) error {
	resp, err := doRequest(args.accessGraph.ExecuteQueryV1WithResponse(ctx, &accessclient.ExecuteQueryV1Params{
		Query: c.access.ls.query,
	}))
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(renderAccessGraphQueryResponse(c.stdout, resp.JSON200))
}

// renderAccessGraphQueryResponse prints a small tabular view of returned nodes
// and falls back to JSON for edge data until a dedicated renderer exists.
func renderAccessGraphQueryResponse(w io.Writer, graph *accessclient.Graph) error {
	if len(*graph.Nodes) == 0 {
		_, err := fmt.Fprintln(w, "No Access Graph resources returned.")
		return trace.Wrap(err)
	}

	var rows [][]string
	for _, node := range *graph.Nodes {
		rows = append(rows, []string{
			string(node.Kind),
			string(node.SubKind),
			string(node.Name),
			node.Id.String(),
		})
	}

	t := asciitable.MakeTableWithTruncatedColumn([]string{"Kind", "Subkind", "Name", "ID"}, rows, "Name")
	if _, err := t.AsBuffer().WriteTo(w); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
