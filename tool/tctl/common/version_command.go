/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"context"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// VersionCommand implements the `tctl version`
type VersionCommand struct {
	app *kingpin.Application

	verCmd *kingpin.CmdClause
}

// Initialize allows VersionCommand to plug itself into the CLI parser.
func (c *VersionCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, _ *servicecfg.Config) {
	c.app = app
	c.verCmd = app.Command("version", "Print the version of your tctl binary.")
}

// TryRun takes the CLI command as an argument and executes it.
func (c *VersionCommand) TryRun(_ context.Context, cmd string, _ commonclient.InitFunc) (match bool, err error) {
	switch cmd {
	case c.verCmd.FullCommand():
		modules.GetModules().PrintVersion()
	default:
		return false, nil
	}

	return true, trace.Wrap(err)
}
