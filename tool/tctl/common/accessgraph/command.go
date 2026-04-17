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
	"log/slog"
	"os"

	"github.com/alecthomas/kingpin/v2"
	accessgraphclient "github.com/gravitational/access-graph/api/client"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
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
	// Access Graph commands bypass the normal tctl auth flow (ApplyConfig), so
	// the logger is never upgraded from its default Warn level. Do it here.
	if c.ccf.Debug {
		utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
	}

	var commandFunc func(context.Context, accessGraphServices) error
	var proxyAddr string

	switch cmd {
	case c.investigate.cmd.FullCommand():
		commandFunc = c.Investigate
	case c.investigate.user.cmd.FullCommand():
		commandFunc = c.InvestigateUser
	case c.investigate.resource.cmd.FullCommand():
		commandFunc = c.InvestigateResource
	case c.access.query.cmd.FullCommand():
		commandFunc = c.AccessQuery
	case c.access.review.resource.cmd.FullCommand():
		commandFunc = c.AccessReviewResource
	case c.access.review.acl.cmd.FullCommand():
		commandFunc = c.AccessReviewACL
	case c.access.review.role.cmd.FullCommand():
		commandFunc = c.AccessReviewRole
	case c.detections.ls.cmd.FullCommand():
		commandFunc = c.DetectionsList
	case c.detections.get.cmd.FullCommand():
		commandFunc = c.DetectionsGet
	case c.accessRequests.ls.cmd.FullCommand():
		commandFunc = c.AccessRequestsList
	case c.accessChanges.ls.cmd.FullCommand():
		commandFunc = c.AccessChangesList
	case c.accessChanges.get.cmd.FullCommand():
		commandFunc = c.AccessChangeGet
	default:
		return false, nil
	}

	creds, err := c.loadAccessGraphCredentials(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	proxyAddr = creds.proxyAddr

	slog.DebugContext(ctx, "Resolved proxy address for Access Graph command", "proxy_addr", proxyAddr)

	if proxyAddr == "" {
		return true, trace.NotFound("proxy public address is not configured")
	}

	if err := c.ensureAccessGraphCert(ctx, creds, clientFunc); err != nil {
		return true, trace.Wrap(err)
	}

	accessGraphClient, err := c.newAccessGraphClient(ctx, proxyAddr, creds.keyRing)
	if err != nil {
		return true, trace.Wrap(err)
	}

	args := accessGraphServices{
		accessGraph: accessGraphClient,
	}

	return true, trace.Wrap(commandFunc(ctx, args))
}
