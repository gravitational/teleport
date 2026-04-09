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

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
)

type accessChangesArgs struct {
	cmd *kingpin.CmdClause
	ls  accessChangesListArgs
}

type accessChangesListArgs struct {
	cmd *kingpin.CmdClause
}

func (c *AccessGraphCommand) initAccessChanges(app *kingpin.Application) {
	c.accessChanges.cmd = app.Command("access-changes", "Monitor access path changes to crown jewels.").Hidden()
	c.initAccessChangesList(c.accessChanges.cmd)
}

func (c *AccessGraphCommand) initAccessChangesList(parent *kingpin.CmdClause) {
	c.accessChanges.ls.cmd = parent.Command("ls", "List access path changes.")
}

// AccessChangesList executes `tctl access-changes ls`.
func (c *AccessGraphCommand) AccessChangesList(context.Context, accessGraphServices) error {
	return trace.NotImplemented("access-changes ls is not implemented")
}
