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
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	accessclient "github.com/gravitational/access-graph/api/client"
	accessgraph "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

const defaultAccessGraphListQuery = "SELECT * FROM nodes"

// AccessGraphCommand implements experimental Access Graph commands.
type AccessGraphCommand struct {
	access   *kingpin.CmdClause
	accessLS *kingpin.CmdClause

	ccf       *tctlcfg.GlobalCLIFlags
	config    *servicecfg.Config
	proxyAddr string
	query     string
}

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, cliFlags *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.ccf = cliFlags
	c.config = config
	c.access = app.Command("access", "Experimental Access Graph commands.")
	c.accessLS = c.access.Command("ls", "List Access Graph resources using the query API.")
	c.accessLS.Flag("query", "SQL query to send to the Access Graph query API.").
		Default(defaultAccessGraphListQuery).
		StringVar(&c.query)
	c.accessLS.Flag("proxy", "Public proxy URL used to reach the Access Graph API. Defaults to the current tsh profile proxy.").
		StringVar(&c.proxyAddr)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AccessGraphCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.accessLS.FullCommand():
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	defer closeFn(ctx)

	pingResp, err := client.Ping(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	proxyAddr := pingResp.GetProxyPublicAddr()
	if proxyAddr == "" {
		return false, trace.NotFound("proxy public address is not configured")
	}

	accessClient, err := c.generateAccessGraphClient(ctx, proxyAddr)
	if err != nil {
		return false, trace.Wrap(err)
	}

	return true, trace.Wrap(c.list(ctx, accessClient))
}

func (c *AccessGraphCommand) list(ctx context.Context, client *accessgraph.ClientWithResponses) error {
	resp, err := doRequest(client.ExecuteQueryV1WithResponse(ctx, &accessclient.ExecuteQueryV1Params{
		Query: c.query,
	}))
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(renderAccessGraphQueryResponse(os.Stdout, resp.JSON200))
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

func isAccessGraphLicensedAndEnabled(pingResp proto.PingResponse) bool {
	features := pingResp.GetServerFeatures()
	entitlement := features.GetEntitlements()[string(entitlements.Policy)]
	licensed := entitlement != nil && entitlement.GetEnabled()
	if !licensed && features.GetPolicy() != nil {
		licensed = features.GetPolicy().GetEnabled()
	}
	if !licensed {
		return false
	}
	return features.GetAccessGraph()
}
