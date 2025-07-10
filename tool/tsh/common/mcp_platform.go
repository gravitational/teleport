// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"github.com/gravitational/teleport/lib/client/mcp/platform"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

// mcpDBStartCommand implements `tsh mcp platform start` command.
type mcpPlatformStartCommand struct {
	*kingpin.CmdClause

	cf                           *CLIConf
	accessRequesterEnabled       bool
	accessRequestReviewerEnabled bool
}

func newMCPPlatformStartCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpPlatformStartCommand {
	cmd := &mcpPlatformStartCommand{
		CmdClause: parent.Command("start", "Start a local MCP server for Teleport APIs."),
		cf:        cf,
	}

	cmd.Flag("access-requester", "Enables tools and resources used to request access to resources.").BoolVar(&cmd.accessRequesterEnabled)
	cmd.Flag("access-requests-reviewer", "Enables tools and resources used to review access requests.").BoolVar(&cmd.accessRequestReviewerEnabled)
	return cmd
}

func (c *mcpPlatformStartCommand) run() error {
	logger, err := initLogger(c.cf, utils.LoggingForMCP, getLoggingOptsForMCPServer(c.cf))
	if err != nil {
		return trace.Wrap(err)
	}

	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Avoid any input request on the command execution. This is required,
	// otherwise the MCP clients will be stuck waiting for a response.
	tc.NonInteractive = false

	clt, err := tc.ConnectToCluster(c.cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	srv, err := platform.NewPlaformServer(platform.PlatformServerConfig{
		Logger:                             logger,
		Client:                             clt.AuthClient,
		Username:                           tc.Username,
		ClusterName:                        tc.SiteName,
		AccessRequesterServerEnabled:       c.accessRequesterEnabled,
		AccessRequestsReviwerServerEnabled: c.accessRequestReviewerEnabled,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(srv.ServeStdio(c.cf.Context, c.cf.Stdin(), c.cf.Stdout()))
}
