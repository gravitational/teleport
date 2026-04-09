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

func (c *AccessGraphCommand) initInvestigate(app *kingpin.Application) {
	c.investigate.cmd = app.Command("investigate", "Investigate identity or resource activity.")
	c.investigate.cmd.Arg("query", "The identity or resource to investigate.").StringVar(&c.investigate.query)
	c.investigate.cmd.Flag("days", "Days range to investigate.").Default("7").IntVar(&c.investigate.days)
}

// Investigate executes `tctl investigate`.
func (c *AccessGraphCommand) Investigate(context.Context, accessGraphServices) error {
	return trace.NotImplemented("investigate is not implemented")
}
