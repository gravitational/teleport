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
	"io"
	"os"

	"github.com/alecthomas/kingpin/v2"
	accessgraphclient "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type accessGraphServices struct {
	accessGraph *accessgraphclient.ClientWithResponses
}

// AccessGraphCommand implements experimental Access Graph commands.
type AccessGraphCommand struct {
	ccf    *tctlcfg.GlobalCLIFlags
	config *servicecfg.Config
	stdout io.Writer

	investigate    investigateArgs
	access         accessArgs
	detections     detectionsArgs
	accessRequests accessRequestsArgs
	accessChanges  accessChangesArgs
}

/*
pctl is a command-line tool for Teleport Identity Security.

Investigate who can access resources, review security detections,
analyze access requests, and monitor access path changes.

  pctl investigate benarent --days=7      What did an identity do?
  pctl detections ls                      List security detections
  pctl access who-can <resource>          Show who can access a resource
  pctl access ls --kind=db                List all databases
  pctl access query "SELECT ..."          Run a graph query
  pctl access-requests ls                 List access requests
  pctl access-changes ls                  List access path changes

Usage:
  pctl [command]

Available Commands:
  access          Analyze who has access to what
  access-changes  Monitor access path changes to crown jewels
  access-requests Review access requests and approvals
  completion      Generate the autocompletion script for the specified shell
  detections      Investigate security detections and anomalies
  help            Help about any command
  investigate     Investigate identity or resource activity

Flags:
  -f, --format string   Output format: text, json, csv, names (default "text")
  -h, --help            help for pctl
      --proxy string    Teleport proxy address (default "teleport-18-ent.teleport.town:443")

Use "pctl [command] --help" for more information about a command.
*/

// Initialize allows AccessGraphCommand to plug itself into the CLI parser.
func (c *AccessGraphCommand) Initialize(app *kingpin.Application, cliFlags *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.ccf = cliFlags
	c.config = config
	if c.stdout == nil {
		c.stdout = os.Stdout
	}

	c.initInvestigate(app)
	c.initAccess(app)
	c.initDetections(app)
	c.initAccessRequests(app)
	c.initAccessChanges(app)
}

// TryRun takes the CLI command as an argument and executes it.
func (c *AccessGraphCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (match bool, err error) {
	var commandFunc func(context.Context, accessGraphServices) error
	var proxyAddr string

	switch cmd {
	case c.investigate.cmd.FullCommand():
		commandFunc = c.Investigate
	case c.access.ls.cmd.FullCommand():
		commandFunc = c.AccessList
	case c.access.whoCan.cmd.FullCommand():
		commandFunc = c.AccessWhoCan
	case c.access.query.cmd.FullCommand():
		commandFunc = c.AccessQuery
	case c.detections.ls.cmd.FullCommand():
		commandFunc = c.DetectionsList
	case c.accessRequests.ls.cmd.FullCommand():
		commandFunc = c.AccessRequestsList
	case c.accessChanges.ls.cmd.FullCommand():
		commandFunc = c.AccessChangesList
	default:
		return false, nil
	}

	client, closeFn, err := clientFunc(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer closeFn(ctx)

	pingResp, err := client.Ping(ctx)
	if err != nil {
		return true, trace.Wrap(err)
	}
	if !isAccessGraphLicensedAndEnabled(pingResp) {
		return true, trace.AccessDenied("Access Graph requires a Teleport Enterprise Auth Server with Access Graph enabled")
	}

	if proxyAddr == "" {
		proxyAddr = pingResp.GetProxyPublicAddr()
	}
	if proxyAddr == "" {
		return true, trace.NotFound("proxy public address is not configured")
	}

	accessGraphClient, err := c.newAccessGraphClient(ctx, proxyAddr)
	if err != nil {
		return true, trace.Wrap(err)
	}

	args := accessGraphServices{
		accessGraph: accessGraphClient,
	}

	return true, trace.Wrap(commandFunc(ctx, args))
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
