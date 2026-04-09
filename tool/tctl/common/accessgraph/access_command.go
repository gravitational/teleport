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

func (c *AccessGraphCommand) initAccess(app *kingpin.Application) {
	c.access.cmd = app.Command("access", "Analyze who has access to what.").Hidden()

	c.initAccessList(c.access.cmd)
	c.initAccessWhoCan(c.access.cmd)
	c.initAccessQuery(c.access.cmd)
}

func (c *AccessGraphCommand) initAccessWhoCan(parent *kingpin.CmdClause) {
	c.access.whoCan.cmd = parent.Command("who-can", "Show which identities have access to a resource.")
	c.access.whoCan.cmd.Arg("resource", "The resource to inspect.").StringVar(&c.access.whoCan.resource)
}

func (c *AccessGraphCommand) initAccessQuery(parent *kingpin.CmdClause) {
	c.access.query.cmd = parent.Command("query", "Run a query against Access Graph.")
	c.access.query.cmd.Arg("query", "The query to execute.").StringVar(&c.access.query.query)
}

// AccessWhoCan executes `tctl access who-can`.
func (c *AccessGraphCommand) AccessWhoCan(context.Context, accessGraphServices) error {
	return trace.NotImplemented("access who-can is not implemented")
}

// AccessQuery executes `tctl access query`.
func (c *AccessGraphCommand) AccessQuery(context.Context, accessGraphServices) error {
	return trace.NotImplemented("access query is not implemented")
}
