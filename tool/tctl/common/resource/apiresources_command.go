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

package resource

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

type APIResourcesCommand struct {
	app    *kingpin.Application
	cmd    *kingpin.CmdClause
	config *servicecfg.Config

	stdout io.Writer
}

func (c *APIResourcesCommand) Initialize(app *kingpin.Application, _ *tctlcfg.GlobalCLIFlags, config *servicecfg.Config) {
	c.app = app
	c.config = config
	c.cmd = app.Command("api-resources", "Lists the tctl-supported resources")

	if c.stdout == nil {
		c.stdout = os.Stdout
	}
}

func (c *APIResourcesCommand) TryRun(ctx context.Context, cmd string, clientFunc commonclient.InitFunc) (bool, error) {
	if cmd != c.cmd.FullCommand() {
		return false, nil
	}

	t := asciitable.MakeTable([]string{"Kind", "Supported Commands", "Singleton", "Description"})
	for kind, handler := range resourceHandlers {
		t.AddRow([]string{
			string(kind),
			strings.Join(handler.supportedCommands(), ","),
			strconv.FormatBool(handler.singleton),
			handler.description,
		})
	}

	t.SortRowsBy([]int{0}, true)

	return true, trace.Wrap(t.WriteTo(c.stdout))
}
