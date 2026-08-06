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

package common

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type mcpLogoutCommand struct {
	*kingpin.CmdClause
	cf *CLIConf
}

func newMCPLogoutCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpLogoutCommand {
	cmd := &mcpLogoutCommand{
		CmdClause: parent.Command("logout", "Remove stored OAuth credentials for an MCP server."),
		cf:        cf,
	}
	cmd.Arg("name", "Name of the MCP server.").Required().SetValue(&cf.AppSQN)
	return cmd
}

// run removes the credentials stored by `tsh mcp login`. It is a purely local
// operation: the app is not looked up in the cluster, so credentials for a
// server that no longer exists can still be removed.
func (c *mcpLogoutCommand) run() error {
	tc, err := makeClient(c.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	credsPath := mcpOAuthTokenPath(c.cf.HomePath, tc.WebProxyHost(), tc.SiteName, c.cf.AppSQN.Name)
	if err := removeMCPOAuthCredentials(credsPath); err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no OAuth credentials stored for MCP server %q", c.cf.AppSQN.Name)
		}
		return trace.Wrap(err)
	}
	fmt.Fprintf(c.cf.Stdout(), "Removed OAuth credentials for MCP server %q.\n", c.cf.AppSQN.Name)
	return nil
}

// removeMCPOAuthCredentials removes the stored credentials file and its
// refresh lock. Returns trace.NotFound when no credentials are stored at the
// path.
func removeMCPOAuthCredentials(path string) error {
	if err := os.Remove(path); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := os.Remove(path + ".lock"); err != nil && !os.IsNotExist(err) {
		return trace.ConvertSystemError(err)
	}
	return nil
}
