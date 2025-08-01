/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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
	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/autoupdate/tools"
)

type autoUpdateCommand struct {
	update *managedUpdatesUpdateCommand
}

func newUpdateCommand(app *kingpin.Application) *autoUpdateCommand {
	root := &autoUpdateCommand{
		update: &managedUpdatesUpdateCommand{},
	}

	root.update.CmdClause = app.Command("update",
		"Update client tools (tsh, tctl) to the latest version defined by the cluster configuration.")
	root.update.CmdClause.Flag("clear", "Removes locally installed client tools updates from the Teleport home directory.").BoolVar(&root.update.clear)

	return root
}

// managedUpdatesUpdateCommand additionally check for the latest available client tools
// version in cluster and runs update.
type managedUpdatesUpdateCommand struct {
	*kingpin.CmdClause
	clear bool
}

func (c *managedUpdatesUpdateCommand) run(cf *CLIConf) error {
	if c.clear {
		toolsDir, err := tools.Dir()
		if err != nil {
			return trace.Wrap(err)
		}
		if err := tools.CleanUp(toolsDir, tools.DefaultClientTools()); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := tools.DownloadUpdate(cf.Context, tc.WebProxyAddr, tc.InsecureSkipVerify); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
