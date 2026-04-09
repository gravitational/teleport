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

func (c *AccessGraphCommand) initDetections(app *kingpin.Application) {
	c.detections.cmd = app.Command("detections", "Investigate security detections and anomalies.").Hidden()
	c.initDetectionsList(c.detections.cmd)
}

func (c *AccessGraphCommand) initDetectionsList(parent *kingpin.CmdClause) {
	c.detections.ls.cmd = parent.Command("ls", "List security detections.")
}

// DetectionsList executes `tctl detections ls`.
func (c *AccessGraphCommand) DetectionsList(context.Context, accessGraphServices) error {
	return trace.NotImplemented("detections ls is not implemented")
}
