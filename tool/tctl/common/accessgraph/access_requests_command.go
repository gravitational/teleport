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

func (c *AccessGraphCommand) initAccessRequests(app *kingpin.Application) {
	c.accessRequests.cmd = app.Command("access-requests", "Review access requests and approvals.").Hidden()
	c.initAccessRequestsList(c.accessRequests.cmd)
}

func (c *AccessGraphCommand) initAccessRequestsList(parent *kingpin.CmdClause) {
	c.accessRequests.ls.cmd = parent.Command("ls", "List access requests.")
}

// AccessRequestsList executes `tctl access-requests ls`.
func (c *AccessGraphCommand) AccessRequestsList(context.Context, accessGraphServices) error {
	return trace.NotImplemented("access-requests ls is not implemented")
}
